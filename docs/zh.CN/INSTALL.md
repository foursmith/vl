# 安装

<p align="center">
  <a href="../INSTALL.md">English</a> · 简体中文
</p>

选择适合你系统的安装方式。

## Homebrew

```sh
brew install foursmith/tap/vl
```

在 macOS 上，用以下命令安装可选工具和 Nerd Font。

```sh
brew install git ripgrep gh resvg font-jetbrains-mono-nerd-font
```

在终端的字体设置中选用 **JetBrainsMono Nerd Font Mono** 来显示文件图标。要审阅 GitHub PR，再运行 `gh auth login`。Markdown 中的远程图片可以用 macOS 自带的 `curl` 加载。

## Go

需要 Go 1.27.3 或更高版本。

```sh
go install github.com/foursmith/vl/cmd/vl@latest
```

## macOS 和 Linux

```sh
curl -fsSL https://github.com/foursmith/vl/releases/latest/download/install.sh | sh
```

安装到 `~/.local/bin`。如果找不到 `vl` 命令，把这个目录加到 `PATH`。

## Windows (PowerShell)

```powershell
Invoke-RestMethod -UseBasicParsing https://github.com/foursmith/vl/releases/latest/download/install.ps1 | Invoke-Expression
```

安装到 `%LOCALAPPDATA%\vl\bin`，并自动加入用户的 `PATH`。

## 从源码构建

需要 Go 1.27.3 或更高版本。

```sh
git clone https://github.com/foursmith/vl.git
cd vl
make build
```

在仓库目录下运行 `./bin/vl`。

## 可选依赖

- 安装 [Nerd Font](https://github.com/ryanoasis/nerd-fonts) 并在终端的字体设置中选用它，以显示文件图标。
- 安装 [Git](https://git-scm.com/)，即可审阅 Git 改动和查看提交历史。
- 安装 [ripgrep](https://github.com/BurntSushi/ripgrep)（`rg`）可加快工作区搜索。
- 安装 [GitHub CLI](https://cli.github.com/) 并运行 `gh auth login`，即可审阅 GitHub PR。
- 安装 [resvg](https://github.com/linebender/resvg) 可预览 SVG 文件，安装 `curl` 可加载 Markdown 中的远程图片。
