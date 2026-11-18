# vl website

Requires Bun and Node.js 22.12 or newer. From this directory:

```sh
bun install --frozen-lockfile
bun run dev
```

`bun run build` prerenders the landing page into `dist/client/` for static hosting.
Run `bun run preview` to serve the production build locally and
`bun run typecheck` to check the TypeScript sources.

## Deploy

Run `bun run deploy` with Wrangler authenticated to the Cloudflare account
that owns `foursmith.com`. The Worker serves the prerendered files in
`dist/client/` at `https://vl.foursmith.com/`.
