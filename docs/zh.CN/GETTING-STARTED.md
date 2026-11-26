# 快速上手

<p align="center">
  <a href="../GETTING-STARTED.md">English</a> · 简体中文
</p>

[安装 VL](INSTALL.md)。要显示文件图标，请安装 [Nerd Font](https://github.com/ryanoasis/nerd-fonts) 并在终端的字体设置中选用它。其他功能所需的工具见[可选依赖](INSTALL.md#可选依赖)。

在终端进入项目目录，运行以下命令。

```sh
vl .
```

如果终端还没选用 Nerd Font，可以用 `vl --icons=false .` 显示纯文本图标。更多启动参数、文件定位和预览用法见[命令行参考](CLI.md)。

**文本文件默认以 Vim 普通模式（Normal）打开。** 按 `i` 开始输入，按 `Esc` 回到普通模式。想关闭 Vim 键位，按 `Ctrl+P`，运行「首选项：切换 Vim 模式」。

1. 在文件树里选一个文件即可预览，按 Enter 把它保留为独立标签页。
2. 按 `Ctrl+G`，用文件名或路径找另一个文件。
3. 如果当前 Git 仓库有改动，按 `Ctrl+3`，选一个改过的文件查看差异。

按 `Ctrl+P` 查找操作，按 `Ctrl+Q` 退出。

## 浏览和搜索代码

- 按 `Ctrl+1` 打开文件树，用 `j`、`k` 移动并预览文件。阅读 Markdown 时，可以用大纲跳到标题。
- 按 `Ctrl+G` 输入文件名或路径。加上 `:行号[:列号]` 就能直接定位，例如 `main.go:42:8`。在文件选择器中按 `F4` 切换是否显示被忽略的文件。
- 按 `Ctrl+F` 查找当前文件，或按 `Ctrl+Shift+F` 搜索工作区。工作区搜索也会查找未保存的改动。打开「搜索详情」可以筛选文件，[安装 ripgrep](https://github.com/BurntSushi/ripgrep) 可以加快搜索。结果导航和替换操作见[查找和替换](KEYBINDINGS.md#查找和替换)。

## 审阅 Git 改动

1. 在 Git 仓库中按 `Ctrl+3` 打开「更改」，选择文件后按 `F2` 把焦点移到 Diff。
2. 按 `[`、`]` 在修改块之间跳转。按 `F7` 打开 Diff 菜单，可以切换内联和并排视图。
3. 在未暂存的 Diff 里按 `s` 暂存当前修改块，在已暂存的 Diff 里按 `s` 取消暂存。使用 Vim 键位时，可以先按 `V` 选中要操作的行。

按 `Ctrl+4` 查看提交历史。要审阅 GitHub PR，先[安装 GitHub CLI](https://cli.github.com/)并运行 `gh auth login`，再在有 GitHub 远程地址的仓库中按 `Ctrl+5`。更多操作见 [Diff 编辑器快捷键](KEYBINDINGS.md#diff-编辑器)。
