# Automatic input method switching

VL can switch to your preferred input method when you return to keyboard navigation.

## Enable switching

1. Press `Ctrl+,` or choose **Preferences: Open Configuration File** from the command palette.
2. Add `autoSwitchInputMethod` under `features`. If `features` already exists, add the setting inside it. The configuration accepts JSONC comments and trailing commas.
3. Save with `Ctrl+S` and run **Reload VL** from the command palette.

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

VL runs the command when navigation resumes after using a text field, leaving Insert mode, or returning to the terminal. It also runs at startup if navigation has focus. Set `enable` to `false` to disable switching.

## Choose a command for your system

The example selects ABC on macOS using [im-select](https://github.com/daipeihust/im-select). Use the executable and arguments for your input method setup, such as `["im-select.exe", "1033"]` on Windows or `["fcitx5-remote", "-c"]` with Fcitx 5 on Linux. The executable must be in `PATH` or given by its absolute path.
