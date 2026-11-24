# vl-preview.yazi

Syntax-highlight source code and text in Yazi with [`vl preview`](https://github.com/foursmith/vl). The preview uses vl's TextMate highlighting and follows the color theme saved in vl, including themes imported from VS Code.

## Install

Use Yazi 26.9.1 or later and [install vl 0.1.3 or later](https://github.com/foursmith/vl#install). Make sure `vl preview --help` works in the shell that launches Yazi.

```sh
ya pkg add foursmith/vl:vl-preview
```

Add the following rules to your `yazi.toml` and restart Yazi.

```toml
[[plugin.prepend_previewers]]
mime = "text/*"
run = "vl-preview"

[[plugin.prepend_previewers]]
mime = "application/{mbox,javascript,wine-extension-ini}"
run = "vl-preview"
```

## Use a VS Code color theme

1. Install the color theme in VS Code.
2. In vl, open the Command Palette with `Ctrl+P` and choose **Appearance: Color Theme**.
3. Choose **+ Import Themes from VS Code**, then select your theme and press `Enter` to save it.
4. Move to a file in Yazi to render a fresh preview with the saved theme.

vl imports installed VS Code color themes, including light, dark, and high-contrast variants. Yazi's own theme controls the surrounding interface. Choose a vl theme that suits your terminal background.
