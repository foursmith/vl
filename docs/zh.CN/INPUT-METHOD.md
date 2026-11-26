# 自动切换输入法

<p align="center">
  <a href="../INPUT-METHOD.md">English</a> · 简体中文
</p>

回到键盘导航时，VL 可以自动切换到指定的输入法。

## 开启自动切换

1. 按 `Ctrl+,`，或在命令面板选择**首选项：打开配置文件**。
2. 在 `features` 下添加 `autoSwitchInputMethod`。如果已有 `features`，把这一项加到现有对象里。配置文件支持 JSONC 注释和尾逗号。
3. 按 `Ctrl+S` 保存，再从命令面板执行**重新加载 VL**。

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

离开输入框、退出 Insert 模式或回到终端后，如果焦点在导航区域，VL 会运行这条命令。启动时焦点在导航区域也会触发。将 `enable` 设为 `false` 即可关闭。

## 按系统选择切换命令

示例使用 [im-select](https://github.com/daipeihust/im-select) 切换到 macOS 的 ABC 输入源。请根据系统填写程序和参数，例如 Windows 的 `["im-select.exe", "1033"]`，或 Linux 上 Fcitx 5 的 `["fcitx5-remote", "-c"]`。程序必须位于 `PATH` 中，也可以填写绝对路径。
