<p align="center">
  <a href="KEYBINDINGS.md">English</a> · 简体中文
</p>

# 键盘快捷键

| 想做什么 | 按键 |
| --- | --- |
| 按名称或路径找文件 | `Ctrl+G`，Go to File |
| 找当前文件里的文字 | `Ctrl+F`，Find |
| 找工作区里的文字 | `Ctrl+Shift+F`，扩大查找范围 |
| 找操作 | `Ctrl+P`，命令面板 |

带 `+` 的键同时按，`gg` 和 `Space w` 等序列依次按。

## 工作台

| 想做什么 | 按键 |
| --- | --- |
| 切换侧栏视图 | `Ctrl+1` 到 `Ctrl+5` 依次对应资源管理器、搜索、更改、提交历史、拉取请求。侧栏有焦点时，`Tab` / `Shift+Tab` 循环切换。 |
| 在侧栏和编辑器之间切换焦点 | `F2`。从侧栏切到编辑器时会保留当前预览标签。 |
| 显示或隐藏侧栏 | `F12` |
| 切换编辑器标签 | `Ctrl+T` 下一个，`Ctrl+Shift+T` 上一个。 |
| 保存或关闭文件 | `Ctrl+S` 保存，`Ctrl+W` 关闭当前标签，`Ctrl+Shift+W` 关闭其他标签。 |
| 切换工作区、打开设置、退出 | `Ctrl+R` 打开最近的工作区，`Ctrl+,` 打开配置文件，`Ctrl+Q` 退出。 |
| 刷新当前视图 | `F5`。在搜索中重跑查询并保留焦点。在编辑区检查磁盘变化并保留未保存的修改。 |
| 查看当前位置能做什么 | `F7` 或鼠标右键打开上下文菜单。 |

关闭标签、切换工作区或退出时，如有未保存的修改会提示。单文件模式下仍可用 `Ctrl+G` 打开同目录的文件，关闭最后一个标签会退出。欢迎页图形可见且主区域有焦点时，方向键可以旋转图形。

### 用空格切标签、保存和关闭

适用于有焦点的编辑区（Vim 普通或可视模式）和图片标签。

| 按键序列 | 操作 |
| --- | --- |
| `Space 1` 到 `Space 9` | 按从左到右的编号切换标签。 |
| `Space ←` / `Space →` | 上一个 / 下一个标签。 |
| `Space w` / `Space q` / `Space z` | 保存 / 关闭 / 保存并关闭。保存适用于可编辑文件。 |
| `Space Esc` | 取消序列。 |

## 在列表里移动

| 要移动多远 | 按键 |
| --- | --- |
| 一项 | `↑` / `↓` 或 `Ctrl+K` / `Ctrl+J`。侧栏和菜单也可用 `k` / `j`。 |
| 半页 / 整页 | `Ctrl+U` / `Ctrl+D` 向上 / 向下半页，`PageUp` / `PageDown` 翻整页。 |
| 第一项 / 最后一项 | `Home` / `End`。侧栏和菜单也可用 `gg` / `G`。 |
| 指定距离或位置 | 侧栏和菜单支持数字前缀。`3j` 下移三项，`5gg` 或 `5G` 跳到第五项。 |

**转到文件**按文件名或路径筛选，不需要 Git。输入 `src/main.go:42:8` 打开文件并定位到第 42 行第 8 列，输入 `:42` 跳到当前文件的第 42 行。工作区中按 `F4` 切换是否显示被忽略的文件，输入 `>` 进入命令面板。

**路径补全**在正文输入 `./` 或 `../` 时出现。用方向键、`Ctrl+K` / `Ctrl+J` 或翻页键选候选，按 `Tab` 插入。

## 查找和替换

| 想做什么 | 文件内 | 工作区 |
| --- | --- | --- |
| 开始查找 | `Ctrl+F`，普通模式也可按 `/`。 | `Ctrl+Shift+F` 或 `Ctrl+2`。在结果列表按 `/` 回到查询框。 |
| 上一处 / 下一处 | 输入框中用 `↑` / `↓` 或 `Ctrl+K` / `Ctrl+J`。编辑器普通模式按 `n` 跳到下一处。 | 列表用通用导航键。输入框中用 `Ctrl+K` / `Ctrl+J` 选结果，焦点留在输入框。 |
| 交回焦点 | 查询框按 `Enter` 定位匹配并回到正文。 | 查询框按 `Enter` 搜索并进入结果。列表按 `Enter` 打开所选结果。 |
| 在输入框间移动 | `Tab` / `Shift+Tab` 切换查找和替换。替换框收起时回到正文。 | `↑` / `↓` 移动到相邻的可见输入框或结果列表。 |
| 替换 | 替换框按 `Enter` 替换当前匹配，先写入编辑缓冲区。 | 替换框按 `Enter` 打开全部替换确认框。可先在列表用 `dd` 或 `Delete` 排除一条结果或整个文件组。 |
| 退出输入 | `Esc` 关闭查找并隐藏高亮。 | `Esc` 回到结果列表。 |

在任一搜索输入框按 `F7`，从菜单进入「搜索选项」，可切换大小写、全字匹配、正则等选项。替换框可用展开箭头打开。文件内查找框有焦点时，`Ctrl+U` / `Ctrl+D` 滚动正文并保留焦点。在工作区搜索输入框按翻页键会进入列表并翻页。

工作区搜索包含尚未保存的编辑内容。展开「搜索详情」可限制已打开或有更改的文件，或填写包含、排除 glob。例如 `*.go, **/*.{ts,tsx}` 匹配多个模式，`./src` 限定在工作区根目录的 `src`。排除优先于包含。搜索和转到文件按 `.rgignore`、`.ignore`、`.gitignore` 的优先级应用忽略规则，搜索时可关闭「遵循忽略规则」或在包含框指定被忽略的目录。替换文本可用 `$1` 或 `${name}` 引用正则捕获组。

## 侧边栏

用 `h` / `←` 选父项并保留折叠状态，用 `l` / `→` 或 `Enter` 展开、折叠或打开所选条目。移动选中项会复用预览标签，打开文件或按 `F2` 可保留预览。用 `e` 从资源管理器或更改列表打开可编辑的原文件。搜索以外的侧栏视图也可用 `r` 或 `R` 刷新。

### 资源管理器

| 按键 | 操作 |
| --- | --- |
| `yy`，然后 `p` | 复制选中的文件或文件夹，再粘贴到选中的目录，或选中文件所在的目录。粘贴前确认路径。 |
| `dd` / `Delete` | 确认后删除所选条目。 |
| `L` | 拆开选中的紧凑文件夹路径。 |
| `o` / `t` | 切换 Markdown 大纲 / 文件时间线焦点。 |

文件操作每次处理一个条目，选中文件夹时也处理子项。复制前先保存源文件。粘贴限当前工作区，保护已有目标，跳过 `.git`，拒绝符号链接和特殊文件。

在大纲或时间线用 `j` / `k` 选择，`Enter` / `l` / `→` 跳到章节或保留历史 Diff，`h` / `←` 收起面板，`Esc` 回到文件树焦点。大纲和时间线交替展开。也可从文件菜单或命令面板打开「查看文件历史」。

### 更改

| 按键 | 操作 |
| --- | --- |
| `s` / `S` | 暂存或取消暂存所选条目 / 仓库。方向由所选条目位于工作区还是暂存区决定。仓库标题有可暂存的更改时优先暂存。 |
| `d` / `D` | 打开确认框，放弃所选条目 / 仓库的未暂存更改。 |
| `c` / `t` | 提交 / 储藏。快捷键打开的储藏操作包含未跟踪文件。 |
| `f` / `p` / `P` | Fetch / 仅快进 Pull / Push。 |

通过上下文菜单或命令面板切换分支、同步、发布和管理储藏。在拉取请求列表按 `o`，可在浏览器中打开所选 PR。

## Diff 编辑器

| 按键 | 操作 |
| --- | --- |
| `[` / `]` | 上一个 / 下一个修改块。 |
| `Enter` | 展开或折叠附近的上下文。 |
| `s` | 暂存或取消暂存选中的更改行，无选区时操作当前修改块。 |
| `S` | 暂存或取消暂存整个文件。 |
| `d` | 确认后放弃选中的更改行或当前修改块。 |
| `D` | 确认后放弃整个文件的未暂存更改。 |
| `F7` | 打开 Diff 操作菜单，也可切换内联与并排布局。 |

文本 Diff 沿用文本编辑器的移动、查找、选区、yank 和 Flash。`[`、`]` 和折叠操作在可视模式之外使用。Git 修改键适用于工作区和暂存区 Diff。并排布局中键盘在右侧导航，鼠标可在两侧选取文本。复制会去掉行号与 Diff 标记。二进制 Diff 按整文件操作，历史和 PR Diff 供查看。

## 图片预览

`+` 或 `=` 放大，`-` 缩小，`0` 恢复适合窗口的大小。缩放后用方向键或 `h j k l` 平移，可加数字前缀。`Ctrl+U` / `Ctrl+D` 上下平移半屏。鼠标滚轮以指针为中心缩放，拖动可平移。点击图片或标签可保留预览。

## 文字编辑

下列组合键作用于有焦点的正文或输入框，在各 Vim 模式下可用。文本 Diff 支持全选和复制。

| 按键 | 操作 |
| --- | --- |
| `Ctrl+A` | 全选。 |
| `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | 复制 / 剪切 / 粘贴系统剪贴板里的文本。 |
| `Ctrl+Z` / `Ctrl+Shift+Z` | 撤销 / 重做。 |
| `Ctrl+Backspace` / `Ctrl+Delete` | 删除到行首 / 行尾。 |

在 Insert 模式或关闭 Vim 时，`Home` / `End` 跳到行首 / 行尾，`Ctrl+←` / `Ctrl+→` 按词移动。正文中，`Ctrl+Home` / `Ctrl+End` 跳到文件首尾，翻屏键与列表相同。按住 `Shift` 移动光标可扩展选区。鼠标拖动选取文本，双击选词，三击选行。

从命令面板运行「切换 Vim 模式」可启用或关闭 Vim。更多操作见 [Vim 指南](VIM.zh-CN.md)。

### Markdown 编辑

在列表项、任务项或引用的行尾按 `Enter` 可续写。编号自动递增，新任务默认未勾选。在空列表项或空引用上再按一次 `Enter` 可退出。

## 菜单和对话框

| 场景 | 移动 | 执行和退出 |
| --- | --- | --- |
| 选择器 | 通用列表导航。 | `Enter` 选择，`Esc` 取消。搜索选项中，筛选框为空时也可用 `Space` 切换选项。 |
| 上下文菜单 | 通用列表导航，也可用 `Tab` / `Shift+Tab`。 | `Enter` / `Space` / `l` / `→` 执行。按右侧显示的快捷键可直达操作。`Esc` / `F7` / `h` / `←` 关闭。 |
| 确认框 | `←` / `→`、`h` / `l` 或 `Tab` / `Shift+Tab` 选按钮，上下移动键与翻页键滚动详情。 | `Enter` / `Space` 执行，`Esc` 取消。未保存更改提示中，`s` 保存，`d` 不保存。 |
| 提交消息等文本对话框 | 通用文字编辑。 | `Enter` 提交，`Esc` 取消。 |

选择器和上下文菜单还可使用 `Ctrl+G`、`Ctrl+P`、`Ctrl+R`（工作区）、`Ctrl+,` 和 `Ctrl+Q`。`Ctrl+[` 在应用中按 `Esc` 处理。

## 终端兼容性

终端可能拦截 `Ctrl+Shift+T`、`Ctrl+Shift+F` 或无法区分 `Ctrl+1` 到 `Ctrl+5`。可从命令面板按名称执行、点击侧栏图标，或在侧栏用 `Tab` / `Shift+Tab` 切换。部分终端会直接复制终端选区，或将 `Ctrl+V` 转为粘贴事件。需要复制 vl 的选区时，也可用编辑器菜单。

## 目前支持以下快捷键自定义

从命令面板执行「首选项：打开快捷键文件」，编辑 `~/.config/vl/keybindings.json`。可直接复制下面的 JSON，里面列出了所有可自定义的快捷键及其默认键位。

```json
[
  {"command": "app.quit", "keys": "Ctrl+Q"},
  {"command": "diff.discardFile", "keys": "Shift+D"},
  {"command": "diff.discardSelectionOrHunk", "keys": "d"},
  {"command": "diff.nextHunk", "keys": "]"},
  {"command": "diff.previousHunk", "keys": "["},
  {"command": "diff.toggleContextLines", "keys": "Enter"},
  {"command": "diff.toggleLayout", "keys": []},
  {"command": "diff.toggleStageFile", "keys": "Shift+S"},
  {"command": "diff.toggleStageSelectionOrHunk", "keys": "s"},
  {"command": "edit.copy", "keys": "Ctrl+C"},
  {"command": "edit.cut", "keys": "Ctrl+X"},
  {"command": "edit.deleteToLineEnd", "keys": "Ctrl+Delete"},
  {"command": "edit.deleteToLineStart", "keys": "Ctrl+Backspace"},
  {"command": "edit.findInFile", "keys": "Ctrl+F"},
  {"command": "edit.paste", "keys": "Ctrl+V"},
  {"command": "edit.redo", "keys": "Ctrl+Shift+Z"},
  {"command": "edit.selectAll", "keys": "Ctrl+A"},
  {"command": "edit.undo", "keys": "Ctrl+Z"},
  {"command": "editor.closeAllTabs", "keys": []},
  {"command": "editor.closeOtherTabs", "keys": "Ctrl+Shift+W"},
  {"command": "editor.closeTab", "keys": "Ctrl+W"},
  {"command": "editor.nextTab", "keys": "Ctrl+T"},
  {"command": "editor.previousTab", "keys": "Ctrl+Shift+T"},
  {"command": "file.save", "keys": "Ctrl+S"},
  {"command": "preferences.openConfig", "keys": "Ctrl+,"},
  {"command": "preferences.openKeybindings", "keys": []},
  {"command": "sidebar.refresh", "keys": ["r", "Shift+R"]},
  {"command": "sidebar.showChanges", "keys": "Ctrl+3"},
  {"command": "sidebar.showCommitHistory", "keys": "Ctrl+4"},
  {"command": "sidebar.showExplorer", "keys": "Ctrl+1"},
  {"command": "sidebar.showPullRequests", "keys": "Ctrl+5"},
  {"command": "sidebar.showSearch", "keys": "Ctrl+2"},
  {"command": "sidebar.toggleFocus", "keys": "F2"},
  {"command": "sidebar.toggleVisibility", "keys": "F12"},
  {"command": "workbench.goToFile", "keys": "Ctrl+G"},
  {"command": "workbench.openCommandPalette", "keys": "Ctrl+P"},
  {"command": "workbench.openContextMenu", "keys": "F7"},
  {"command": "workbench.refresh", "keys": "F5"},
  {"command": "workspace.findInFiles", "keys": "Ctrl+Shift+F"},
  {"command": "workspace.openRecent", "keys": "Ctrl+R"}
]
```

保存后，在命令面板执行「重新加载 VL」。

省略配置项时使用默认键位。`keys` 可以填写单键、组合键或多个键位组成的数组。设为 `[]` 时，该操作没有快捷键。键名不区分大小写，按住 Shift 的组合键须写成 `Shift+S` 这样的形式。

快捷键沿用对应操作的焦点规则。遇到未知命令、无效键位或自定义 `when` 条件，vl 会提示并跳过该条配置。
