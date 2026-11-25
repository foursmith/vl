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

VS Code 风格的终端工作台，支持 Vim 键位。
浏览代码、审查 diff，顺手改几行。

![](./assets/workbench.webp)

现在我的代码都交给 Agent 写，提交前我还是会过一遍 diff。
VS Code 加 Vim 扩展是我用了好几年的组合。做 vl 时，我挑的都是自己平时用得上的功能。
整个工具就一个约 10 MB 的二进制文件，拿来就能跑。

- **省掉的 VS Code 功能**
  - 语言服务器集成
    - 代码补全
    - 诊断
    - 跳转到定义
  - 调试
  - 集成终端
  - 构建和测试任务
  - 扩展运行环境和扩展市场
- **省掉的 Vim 功能**
  - 可视块模式
  - 宏
  - 命名寄存器
  - 标记与标记跳转
  - Ex 命令
  - Vimscript 配置和插件

## 安装

### Homebrew

```sh
brew tap foursmith/tap
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

工作台的操作见[键盘快捷键](docs/KEYBINDINGS.zh-CN.md)。

### 自动切换输入法

按 `Ctrl+,` 或在命令面板选择**首选项：打开配置文件**。配置文件支持 JSONC 注释和尾逗号。添加以下字段，按 `Ctrl+S` 保存，再从命令面板执行**重新加载 VL**。

```json
{
  "autoSwitchInputMethod": {
    "enable": true,
    "cmd": ["im-select", "com.apple.keylayout.ABC"]
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
