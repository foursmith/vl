# Yazi integration

<p align="center">
  English · <a href="zh.CN/YAZI.md">简体中文</a>
</p>

Use VL 0.1.3 or newer. Check that `vl preview --help` works in the shell that launches Yazi.

## Preview code

Requires Yazi 26.9.1 or newer. Install the preview plugin.

```sh
ya pkg add foursmith/vl:vl-preview
```

Add these rules to `yazi.toml` and restart Yazi to enable code previews.

```toml
[[plugin.prepend_previewers]]
mime = "text/*"
run = "vl-preview"

[[plugin.prepend_previewers]]
mime = "application/{mbox,javascript,wine-extension-ini}"
run = "vl-preview"
```

## Open text files

Add this to `yazi.toml` to open text files in VL with Enter or `o`.

```toml
[[opener.vl]]
run = "vl -- %s1"
block = true
desc = "Edit in vl"

[[open.prepend_rules]]
mime = "text/*"
use = ["vl", "reveal"]

[[open.prepend_rules]]
mime = "application/{json,ndjson,mbox,javascript,wine-extension-ini}"
use = ["vl", "reveal"]
```
