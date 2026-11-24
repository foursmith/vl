package editor

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/foursmith/vl/internal/config"
)

const maxHanWordRunes = 6

// HanDictionary supplies the word frequencies for optional Chinese word motions.
type HanDictionary struct {
	scores       map[string]float64
	unknown      float64
	unknownWords func() *hanHMM
}

var installedHanDictionary = sync.OnceValue(func() *HanDictionary {
	dir, err := config.Dir()
	if err != nil {
		return nil
	}
	return loadHanDictionaryFromDir(dir)
})

// InstalledHanDictionary loads the user's dictionary once, on first use.
// It returns nil if the dictionary is unavailable.
func InstalledHanDictionary() *HanDictionary { return installedHanDictionary() }

func loadHanDictionaryFromDir(dir string) *HanDictionary {
	for _, name := range []string{"dict.txt", "dict.txt.small", "dict.txt.big"} {
		if dict := loadHanDictionary(filepath.Join(dir, name)); dict != nil {
			return dict
		}
	}
	return nil
}

func loadHanDictionary(path string) *HanDictionary {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	scores := make(map[string]float64)
	var total float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var word []byte
		for field := range bytes.FieldsSeq(scanner.Bytes()) {
			if word == nil {
				word = field
				continue
			}
			frequency, err := strconv.Atoi(string(field))
			if err == nil && frequency > 0 && keepHanWord(word, frequency) {
				old := scores[string(word)]
				if score := float64(frequency); score > old {
					total += score - old
					scores[string(word)] = score
				}
			}
			break
		}
	}
	if scanner.Err() != nil || total == 0 {
		return nil
	}
	normalizer := math.Log(total)
	for word, score := range scores {
		scores[word] = math.Log(score) - normalizer
	}
	modelPath := filepath.Join(filepath.Dir(path), "hmm_model.utf8")
	return &HanDictionary{
		scores:       scores,
		unknown:      -normalizer,
		unknownWords: sync.OnceValue(func() *hanHMM { return loadHanHMM(modelPath) }),
	}
}

// Retain common words and editor terms so the full upstream dictionary does
// not become a resident index of uncommon words.
func keepHanWord(word []byte, frequency int) bool {
	if len(word) > 3*maxHanWordRunes {
		return false
	}
	if frequency < 100 && len(word) != 3 {
		switch string(word) {
		case "分词", "分詞", "繁体", "繁體", "光标", "光標", "游标", "游標", "遊標", "词界", "詞界",
			"字符集", "字符串", "字串", "源代码", "源代碼", "开源", "開源", "输入法", "輸入法",
			"命令行", "命令列", "自然语言", "自然語言":
		default:
			return false
		}
	}
	for _, r := range string(word) {
		if r < '\u3400' || r > '\u9fff' {
			return false
		}
	}
	return true
}

// hanWordRun caches the most recently visited sequence of Han characters.
// Each rune stores its distance from the start of the word containing it.
type hanWordRun struct {
	start int
	words []uint8
}

func (run *hanWordRun) class(dict *HanDictionary, text string, pos, offset int) int {
	if pos < run.start || pos-run.start >= len(run.words) {
		start, end := offset, offset
		run.start = pos
		for start > 0 {
			r, size := utf8.DecodeLastRuneInString(text[:start])
			if !unicode.Is(unicode.Han, r) {
				break
			}
			start -= size
			run.start--
		}
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if !unicode.Is(unicode.Han, r) {
				break
			}
			end += size
		}
		run.words = dict.segment([]rune(text[start:end]))
	}
	// Use negative classes to identify each word by its absolute start.
	return ^(pos - int(run.words[pos-run.start]))
}

func (dict *HanDictionary) segment(runes []rune) []uint8 {
	next := make([]uint8, len(runes))
	// At each boundary, choose the highest-scoring path to the end of the
	// run. Only scores within one dictionary word of the cursor are needed.
	var scores [maxHanWordRunes + 1]float64
	var spelling [maxHanWordRunes * utf8.UTFMax]byte
	for start := len(runes) - 1; start >= 0; start-- {
		bestEnd := start + 1
		best := dict.unknown + scores[bestEnd%len(scores)]
		word := spelling[:0]
		for end := start + 1; end <= min(start+maxHanWordRunes, len(runes)); end++ {
			word = utf8.AppendRune(word, runes[end-1])
			if score, ok := dict.scores[string(word)]; ok {
				score += scores[end%len(scores)]
				if score >= best {
					best, bestEnd = score, end
				}
			}
		}
		scores[start%len(scores)] = best
		next[start] = uint8(bestEnd - start)
	}
	for start := 0; start < len(next); {
		end := start + int(next[start])
		if end == start+1 {
			for end < len(next) && next[end] == 1 {
				end++
			}
			clear(next[start:end])
			if end-start > 1 {
				dict.refineSingletons(runes[start:end], next[start:end])
			}
			start = end
			continue
		}
		for i := start; i < end; i++ {
			next[i] = uint8(i - start)
		}
		start = end
	}
	return next
}

func (dict *HanDictionary) refineSingletons(runes []rune, offsets []uint8) {
	// Respect the frequency-based choice when the entire run is in the dictionary.
	if len(runes) <= maxHanWordRunes {
		if _, exists := dict.scores[string(runes)]; exists {
			return
		}
	}
	if model := dict.unknownWords(); model != nil {
		model.refine(runes, offsets)
	}
}
