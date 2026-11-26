# 命令行

## 打开工作区或文件

| 命令 | 用途 |
| --- | --- |
| `vl` | 重新打开最近一个仍存在的工作区。没有可用的记录时，运行 `vl .`。 |
| `vl .` | 把当前目录作为工作区打开。 |
| `vl path/to/project` | 打开指定目录。 |
| `vl path/to/file` | 在编辑器中打开单个文件。 |
| `vl path/to/file:42:8` | 打开文件并定位到第 42 行第 8 列。行号和列号从 1 开始，列号可以省略。 |

选项放在路径前。路径含空格时加引号，例如 `vl "my project"`。如果文件或目录就叫 `preview`，用 `vl -- preview` 打开。

## 启动选项

| 选项 | 用途 |
| --- | --- |
| `-h`、`--help` | 显示命令行帮助。 |
| `-v`、`--version` | 显示版本。 |
| `--lang en`、`--lang zh-CN` | 设置并保存界面语言。未设置时使用英语。 |
| `--icons=false` | 使用纯文本图标，适合终端没有选用 Nerd Font 时。 |
| `--theme ID` | 按 ID 选用内置或已导入的主题，例如 `dark_modern` 或 `light_modern`。 |

## 在 Rex 中恢复工作区

在支持 OSC 7577 的 Rex Server 中，Server 重启后，vl 可以在原来的分屏重新打开当前工作区，恢复已保存的标签页、侧栏视图和文件位置。恢复时使用已保存的偏好和默认启动选项。

在 Rex 的 `init.lua` 中添加下面的规则，允许恢复家目录下的工作区。如果项目放在其他位置，把 `~` 换成项目的上级目录。确认 shell 的 `PATH` 中有 `vl`，再运行 `rex config reload` 应用配置。 [Rex 恢复规则](https://www.superlogical.com/rex/docs/customize/resume-rules#adding-a-rule)

```lua
rex.set("resume.trust.vl", {
  command = { "vl", rex.match.under("~") },
})
```

## 输出文件预览

`vl preview` 把文件内容输出到标准输出。在本仓库的根目录运行以下命令，可输出第 41 到 60 行并显示原始行号。

```sh
vl preview --skip 40 --lines 20 --line-numbers cmd/vl/main.go
```

预览选项放在 `preview` 之后、文件路径之前。

| 选项 | 用途 |
| --- | --- |
| `-h`、`--help` | 显示预览命令的帮助。 |
| `--skip N` | 跳过开头的 `N` 行，默认为 `0`。 |
| `--lines N` | 最多输出 `N` 行源文件内容，默认值 `0` 表示输出到文件末尾。 |
| `--width N` | 每行最多输出 `N` 列，包含行号。默认值 `0` 表示不限宽。 |
| `--wrap` | 到达指定宽度时换行，要求 `--width` 至少为 `2`。 |
| `--line-numbers` | 显示源文件行号。 |
| `--tab-size N` | 设置格式化输出中的制表符宽度，范围是 `1` 到 `32`，默认为 `4`。 |
| `--color MODE` | 可选 `auto`（默认）、`always` 或 `never`。通过管道传递输出时可用 `always` 保留 ANSI 颜色。 |
| `--theme ID` | 彩色输出使用的语法主题，默认跟随当前亮暗配色对应的已保存主题。 |

要指定文件的高亮语言，可以点击编辑器状态栏中的语言名称，或运行「更改语法高亮」。VL 会按文件保存这项设置，`vl preview` 也会使用它。
