import type { JSX } from "@solidjs/web"
import { createRootRoute, HeadContent, Scripts } from "@tanstack/solid-router"
import stylesheet from "../styles.css?url"

const title = "vl — Terminal workbench with Vim keybindings"
const description = "A VSCode-like terminal workbench with Vim-like keybindings. Browse code, review diffs, and make quick edits."
const siteUrl = "https://vl.foursmith.com/"
const imageUrl = new URL("og.png", siteUrl).href
const imageAlt = "Blue ANSI VL wordmark in the vl terminal workbench."

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1" },
      { name: "theme-color", content: "#0e0f11" },
      { title },
      { name: "description", content: description },
      { property: "og:type", content: "website" },
      { property: "og:site_name", content: "vl" },
      { property: "og:url", content: siteUrl },
      { property: "og:title", content: title },
      { property: "og:description", content: description },
      { property: "og:image", content: imageUrl },
      { property: "og:image:type", content: "image/png" },
      { property: "og:image:width", content: "1200" },
      { property: "og:image:height", content: "630" },
      { property: "og:image:alt", content: imageAlt },
      { name: "twitter:card", content: "summary_large_image" },
      { name: "twitter:title", content: title },
      { name: "twitter:description", content: description },
      { name: "twitter:image", content: imageUrl },
      { name: "twitter:image:alt", content: imageAlt },
    ],
    links: [
      { rel: "icon", type: "image/png", sizes: "96x96", href: "/favicon.png" },
      { rel: "canonical", href: siteUrl },
      { rel: "stylesheet", href: stylesheet },
    ],
    scripts: [
      {
        type: "application/ld+json",
        children: JSON.stringify({ "@context": "https://schema.org", "@type": "WebSite", name: "vl", url: siteUrl }),
      },
    ],
  }),
  shellComponent: RootDocument,
})

function RootDocument(props: { children: JSX.Element }) {
  return (
    <html lang="en">
      <head><HeadContent /></head>
      <body>
        {props.children}
        <Scripts />
      </body>
    </html>
  )
}
