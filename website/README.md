# vl website

Requires Bun and Node.js 22.12 or newer. From this directory:

```sh
bun install --frozen-lockfile
bun run dev
```

`bun run build` prerenders both languages into `dist/client/` and builds the Pagefind search index.
Run `bun run preview` to serve the production build locally and
`bun run typecheck` to check the TypeScript sources.

Add documentation to `../docs/` and its Chinese translation to `../docs/zh.CN/` with the same filename.
Register the page in `src/docs-paths.ts` to add it to navigation, the sitemap, and the build.

## Deploy

Run `bun run deploy` with Wrangler authenticated to the Cloudflare account
that owns `foursmith.com`. The Worker serves the prerendered files in
`dist/client/` at `https://vl.foursmith.com/`.
