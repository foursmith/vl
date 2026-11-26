# 中文分词

<p align="center">
  <a href="../CHINESE-WORD-SEGMENTATION.md">English</a> · 简体中文
</p>

开启后，Vim 按词移动、`Ctrl+←` / `Ctrl+→` 和鼠标双击选词都会按中文词语的边界操作。关闭 Vim 模式时，方向键快捷键和双击选词仍然适用。

## 开启中文分词

1. 任选一个 Jieba 词典，下载到 `~/.config/vl/extra/`。
2. 在 `~/.config/vl/vl.config.json` 的 `features` 下添加 `chineseWordSegmentation`。如果已有 `features`，把这一项加到现有对象里。
3. 保存配置并重开 VL。

```json
{
  "features": {
    "chineseWordSegmentation": true
  }
}
```

## 选择词典

如果目录里有多个版本，VL 会按表格从上到下的顺序使用第一个可用文件。

| 词典 | 下载大小 | 词汇覆盖 |
| --- | ---: | --- |
| [`dict.txt`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/jieba/dict.txt) | 4.84 MiB | Jieba 默认使用的标准词典。 |
| [`dict.txt.small`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/extra_dict/dict.txt.small) | 1.48 MiB | 词条最少，下载文件也最小。 |
| [`dict.txt.big`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/extra_dict/dict.txt.big) | 8.19 MiB | 词条最多，繁体词汇覆盖更广。 |

如果还想识别生词，可以把 [CppJieba 的 `hmm_model.utf8`](https://raw.githubusercontent.com/yanyiwu/cppjieba/v5.6.7/dict/hmm_model.utf8) 下载到同一目录。
