# vl website

Requires Bun and Node.js 22.12 or newer. From this directory:

```sh
bun install --frozen-lockfile
bun run dev
```

`bun run build` prerenders both languages and their document search data into `dist/client/`.
Run `bun run preview` after building to serve the static files and test Cloudflare redirects locally.
Run `bun run typecheck` to check the TypeScript sources.

Add documentation to `../docs/` and its Chinese translation to `../docs/zh.CN/` with the same filename.
Register the page in `src/docs-paths.ts` to add it to navigation, the sitemap, and the build.

## Deploy

Run `bun run deploy` with Wrangler authenticated to the Cloudflare account
that owns `foursmith.com`. The Worker serves the prerendered files in
`dist/client/` at `https://vl.foursmith.com/`.
