# Yazi 集成

<p align="center">
  <a href="../YAZI.md">English</a> · 简体中文
</p>

使用 VL 0.1.3 或更高版本，并确认启动 Yazi 的 shell 能运行 `vl preview --help`。

## 预览代码

需要 Yazi 26.9.1 或更高版本。先安装预览插件。

```sh
ya pkg add foursmith/vl:vl-preview
```

在 `yazi.toml` 中添加以下配置，然后重启 Yazi 来启用代码预览。

```toml
[[plugin.prepend_previewers]]
mime = "text/*"
run = "vl-preview"

[[plugin.prepend_previewers]]
mime = "application/{mbox,javascript,wine-extension-ini}"
run = "vl-preview"
```

## 打开文本文件

在 `yazi.toml` 中添加以下配置，就能按 Enter 或 `o` 用 VL 打开文本文件。

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
