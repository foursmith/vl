import { stemmer } from "stemmer"
import type { SearchDocument } from "./docs-paths"
import { localizedPath, searchCorpusPath, type Locale } from "./locales"

const technical = /--[a-z][\w-]*|(?:(?:ctrl|alt|shift|meta|cmd)\+)+(?:[a-z0-9]+|[^\s\w])|[\w]+(?:[./_-][\w]+)+/gi
const han = /\p{Script=Han}/u
const word = /^[a-z]+$/
const prefixable = /^(?:[a-z0-9]{3,}|--[a-z][a-z0-9-]{2,})$/

export type SearchResult = { url: string; title: string; heading: string; excerpt: string }

type DocumentSearch = (query: string) => SearchResult[]
const searches = new Map<Locale, Promise<DocumentSearch>>()

export function loadDocumentSearch(locale: Locale): Promise<DocumentSearch> {
  let pending = searches.get(locale)
  if (!pending) {
    pending = fetch(localizedPath(searchCorpusPath, locale))
      .then((response) => {
        if (!response.ok) throw new Error(`Document search: ${response.status}`)
        return response.json() as Promise<SearchDocument[]>
      })
      .then((corpus) => createDocumentSearch(locale, corpus))
      .catch((error: unknown) => {
        searches.delete(locale)
        throw error
      })
    searches.set(locale, pending)
  }
  return pending
}

function normalize(value: string) {
  return value.normalize("NFKC").toLowerCase()
}

function createTokenizer(locale: Locale) {
  const segmenter = typeof Intl.Segmenter === "function" ? new Intl.Segmenter(locale, { granularity: "word" }) : undefined

  return (text: string) => {
    const terms: string[] = []
    let cursor = 0

    function addWords(value: string) {
      const words = segmenter
        ? Array.from(segmenter.segment(value), ({ segment, isWordLike }) => isWordLike ? segment : "").filter(Boolean)
        : value.match(/\p{Script=Han}|[\p{L}\p{N}]+/gu) ?? []
      for (const term of words) terms.push(word.test(term) ? stemmer(term) : term)
    }

    for (const match of text.matchAll(technical)) {
      const term = match[0]
      addWords(text.slice(cursor, match.index))
      terms.push(term)
      const basename = term.slice(term.lastIndexOf("/") + 1)
      if (basename && basename !== term) terms.push(basename)
      cursor = match.index + term.length
    }
    addWords(text.slice(cursor))
    return new Set(terms)
  }
}

function createDocumentSearch(locale: Locale, corpus: SearchDocument[]): DocumentSearch {
  const tokenize = createTokenizer(locale)
  const searchable = (value: string) => {
    const text = normalize(value)
    return { text, terms: tokenize(text) }
  }

  const rows = corpus.flatMap(([path, title, sections]) => {
    const page = searchable(title)
    return sections.map(([fragment, heading, text], index) => ({
      path, fragment, title, heading, text, page,
      pageEntry: index === 0,
      section: searchable(heading),
      body: searchable(text),
    }))
  })

  function strength(field: ReturnType<typeof searchable>, term: string, prefix: boolean) {
    if (field.terms.has(term)) return 1
    if (han.test(term) && field.text.includes(term)) return 0.5
    if (prefix) {
      for (const word of field.terms) {
        if (word.startsWith(term)) return 0.5
      }
    }
    return 0
  }

  return (query) => {
    const normalized = normalize(query.trim())
    const terms = [...tokenize(normalized)]
    if (!terms.length) return []

    const prefixes = terms.map((term, index) => index === terms.length - 1 && prefixable.test(term))
    const rarity = terms.map((term) => {
      const count = rows.filter(({ page, section, body }) =>
        strength(page, term, false) || strength(section, term, false) || strength(body, term, false),
      ).length
      return Math.log((rows.length + 1) / (count + 1))
    })
    const scored = rows.map((row) => {
      let coverage = 0
      let score = 0
      let rarityScore = 0
      for (const [index, term] of terms.entries()) {
        const heading = strength(row.section, term, prefixes[index])
        const title = strength(row.page, term, prefixes[index])
        const body = strength(row.body, term, prefixes[index])
        if (!(heading || title || body)) continue
        coverage++
        rarityScore += rarity[index]
        score += 8 * heading + (row.pageEntry ? 8 : 2) * title + body
      }
      if (row.section.text.includes(normalized)) score += 4
      if (row.body.text.includes(normalized)) score += 2
      return { row, coverage, score, rarityScore }
    })

    const complete = scored.filter((hit) => hit.coverage === terms.length)
    // Chinese queries sometimes contain a common word absent from the intended section.
    const partial = !complete.length && han.test(normalized) && !normalized.match(technical)
    const matches = partial ? scored.filter((hit) => hit.coverage > 0) : complete
    matches.sort((a, b) => b.coverage - a.coverage ||
      (partial ? b.rarityScore - a.rarityScore : 0) || b.score - a.score)

    const seen = new Set<string>()
    const results: SearchResult[] = []
    for (const { row } of matches) {
      if (seen.has(row.path)) continue
      seen.add(row.path)
      results.push({
        url: row.fragment ? `${row.path}#${row.fragment}` : row.path,
        title: row.title,
        heading: row.heading,
        excerpt: excerpt(row.text, normalized, terms),
      })
      if (results.length === 8) break
    }
    return results
  }
}

function excerpt(text: string, query: string, terms: string[]) {
  const lower = text.toLowerCase()
  let at = lower.indexOf(query)
  if (at < 0) {
    const offsets = terms.map((term) => lower.indexOf(term)).filter((offset) => offset >= 0)
    at = offsets.length ? Math.min(...offsets) : 0
  }
  let start = Math.max(0, at - 42)
  const nextSpace = text.indexOf(" ", start)
  if (start && nextSpace >= start && nextSpace < at) start = nextSpace + 1
  const end = Math.min(text.length, start + 160)
  return `${start ? "…" : ""}${text.slice(start, end).trim()}${end < text.length ? "…" : ""}`
}
