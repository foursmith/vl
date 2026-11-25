```
░▒▒▒▒░       ▒▒▒▒▒   ▒▒▒▒▒
░▒▓▓▓▓      ▓▓▓▓▓▓   ▒▓▓▓▓
░░▒▓▓▓▓    ▒▓▓▓▓▓▓   ▒▓▓▓▓
░░░▒▓▓▓▒  ░▓▓▓▓▓▓░   ▒▓▓▓▓
 ░░░▒▓▓▓░░▓▓▓▓▓▓░    ▓▓▓▓▓
  ░░░▓▓▓▓▓▓▓▓▓▓▒     ▓▓▓▓▓░░░░░░░░░
   ░░░▓▓▓▓▓▓▓▓▒      ▓▓▓▓▓▓▓▓▓▓▓▓▓▒
    ░░▒▒▒▒▒▒▓▒       ▒▒▒▒▒▒▒▒▒▒▒▒▒▒
     ░▒▒▒▒▒▒▓        ▒▒▒▒▒▒▒▒▒▒▒▒▒▒
```

<p align="center">
  <a href="README.md">English</a> · 简体中文
</p>

**在终端里像用 VSCode 一样读代码、审阅 Git 改动，顺手改几行。**

在项目目录里运行 `vl .` 就能开始。

## 特色

- 熟悉的 VSCode 界面和体验，但是更快更小，没有任何多余的功能, 只需要 10MB 的二进制文件。
- 丝滑的鼠标操作搭配高效的 [VIM 键位](./docs/VIM.zh-CN.md)，并且针对中文用户进行专门优化。

## 安装

### Homebrew

```sh
brew install foursmith/tap/vl
```

### Go

需要 Go 1.27.3 或更高版本。

```sh
go install github.com/foursmith/vl/cmd/vl@latest
```

### macOS 和 Linux

```sh
curl -fsSL https://github.com/foursmith/vl/releases/latest/download/install.sh | sh
```

安装到 `~/.local/bin`。

### Windows (PowerShell)

```powershell
Invoke-RestMethod -UseBasicParsing https://github.com/foursmith/vl/releases/latest/download/install.ps1 | Invoke-Expression
```

安装到 `%LOCALAPPDATA%\vl\bin`，并自动加入用户的 `PATH`。

### 从源码构建

需要 Go 1.27.3 或更高版本。

```sh
git clone https://github.com/foursmith/vl.git
cd vl
make build
```

在仓库目录下运行 `./bin/vl`。

## 用法

### 可选依赖

- Git 仓库功能需要 Git。[Git](https://git-scm.com/)
- 安装 ripgrep（`rg`）可加快工作区搜索。[ripgrep](https://github.com/BurntSushi/ripgrep)
- GitHub 拉取请求需要 `gh`，安装后运行 `gh auth login`。[GitHub CLI](https://cli.github.com/)
- Markdown 远程图片需要 `curl`。
- SVG 预览需要 `resvg`。[resvg](https://github.com/linebender/resvg)
- 文件图标需要在终端中选用 Nerd Font。[Nerd Fonts](https://github.com/ryanoasis/nerd-fonts)

### 运行模式

- `vl .` 打开工作台。`Ctrl+G` 按名称或路径找文件，`Ctrl+Shift+F` 搜索文件内容。
- `vl path/to/file` 在编辑器中打开单个文件。
- `vl preview path/to/file` 输出带语法高亮的代码。

点击状态栏中的语言，或在命令面板选择**更改语法高亮**。设置按文件保存，也适用于 `vl preview`。**自动检测**根据文件名或 shebang 判断语言，**纯文本**关闭高亮。

想换键位时，在命令面板执行「首选项：打开快捷键文件」。 [默认快捷键和配置示例](docs/KEYBINDINGS.zh-CN.md)

### 自动切换输入法

按 `Ctrl+,` 或在命令面板选择**首选项：打开配置文件**。配置文件支持 JSONC 注释和尾逗号。添加以下字段，按 `Ctrl+S` 保存，再从命令面板执行**重新加载 VL**。

```json
{
  "features": {
    "autoSwitchInputMethod": {
      "enable": true,
      "cmd": ["im-select", "com.apple.keylayout.ABC"]
    }
  }
}
```

离开输入框、退出 Insert 模式或回到终端后，如果焦点在导航区域，vl 会运行这条命令。启动时焦点在导航区域也会触发。将 `enable` 设为 `false` 即可关闭。

示例使用 [im-select](https://github.com/daipeihust/im-select) 切换到 macOS 的 ABC 输入源。请根据系统填写程序和参数，例如 Windows 的 `["im-select.exe", "1033"]`，或 Linux 上 Fcitx 5 的 `["fcitx5-remote", "-c"]`。程序必须位于 `PATH` 中，也可以填写绝对路径。

### Yazi 集成

#### 预览代码

需要 Yazi 26.9.1 或更高版本。先安装预览插件。

```sh
ya pkg add foursmith/vl:vl-preview
```

在 `yazi.toml` 中添加以下配置，启用代码预览。

```toml
[[plugin.prepend_previewers]]
mime = "text/*"
run = "vl-preview"

[[plugin.prepend_previewers]]
mime = "application/{mbox,javascript,wine-extension-ini}"
run = "vl-preview"
```

#### 打开文本文件

在 `yazi.toml` 中添加以下配置，就能按 Enter 或 `o` 用 vl 打开文本文件。

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

## 开发

```sh
make build  # 更新 bin/vl
make test
make check  # 运行 go vet 和启用竞态检测的测试
```

## 致谢

- 感谢 [TTT](https://github.com/eugenioenko/ttt) 和 [textmate-go](https://github.com/eugenioenko/textmate-go) 提供语法高亮支持和实现参考。
- 工作台设计参考 [VS Code](https://github.com/microsoft/vscode)，键位参考 [Vim](https://github.com/vim/vim)。

## 许可证

本源代码受 Mozilla Public License 2.0（MPL 2.0）条款约束。如果随附文件中没有 MPL 副本，可以前往 [MPL 2.0 官网](https://mozilla.org/MPL/2.0/)获取。

完整条款见 [LICENSE](LICENSE)。第三方组件沿用各自的许可证，详见[第三方声明](THIRD_PARTY_NOTICES.md)。
