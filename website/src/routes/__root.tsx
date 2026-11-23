import { NoHydration, type JSX } from "@solidjs/web"
import { createRootRoute, HeadContent, Scripts, useLocation } from "@tanstack/solid-router"
import { localizedPath, localeForPath, locales } from "../locales"
import { siteUrl } from "../docs-paths"

export const Route = createRootRoute({
  head: ({ matches }) => {
    const locale = localeForPath(matches.at(-1)?.pathname ?? "/")
    const imageUrl = new URL(locale === "en" ? "og.png?v=3" : "og.zh-CN.png", siteUrl).href
    const { siteTitle: title, description, imageAlt } = locales[locale]
    return {
      meta: [
        { name: "viewport", content: "width=device-width, initial-scale=1" },
        { name: "theme-color", content: "#181818" },
        { title },
        { name: "description", content: description },
        { property: "og:type", content: "website" },
        { property: "og:site_name", content: "VL" },
        { property: "og:url", content: new URL(localizedPath("/", locale), siteUrl).href },
        { property: "og:locale", content: locale === "en" ? "en_US" : "zh_CN" },
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
      ],
      scripts: [
        {
          type: "application/ld+json",
          children: JSON.stringify({ "@context": "https://schema.org", "@type": "WebSite", name: "VL", url: siteUrl }),
        },
      ],
    }
  },
  shellComponent: RootDocument,
})

function RootDocument(props: { children: JSX.Element }) {
  const locale = useLocation({ select: (location) => localeForPath(location.pathname) })

  return (
    <NoHydration>
      <html lang={locale()}>
        <head>
          <meta charset="utf-8" />
          <HeadContent />
        </head>
        <body>
          {props.children}
          <Scripts />
        </body>
      </html>
    </NoHydration>
  )
}
