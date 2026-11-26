# Chinese word segmentation

Use Chinese word boundaries for Vim word motions, `Ctrl+Left` / `Ctrl+Right`, and double-click selection. The arrow shortcuts and double-click selection also work with Vim mode off.

## Enable segmentation

1. Download one Jieba dictionary into `~/.config/vl/extra/`.
2. Add `chineseWordSegmentation` under `features` in `~/.config/vl/vl.config.json`. Merge it into the existing `features` object if present.
3. Save the configuration and reopen VL.

```json
{
  "features": {
    "chineseWordSegmentation": true
  }
}
```

## Choose a dictionary

If more than one dictionary is present, VL checks the table from top to bottom and uses the first usable file.

| Dictionary | Download size | Vocabulary |
| --- | ---: | --- |
| [`dict.txt`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/jieba/dict.txt) | 4.84 MiB | Jieba's standard dictionary. |
| [`dict.txt.small`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/extra_dict/dict.txt.small) | 1.48 MiB | The fewest entries and the smallest download. |
| [`dict.txt.big`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/extra_dict/dict.txt.big) | 8.19 MiB | The most entries, with broader coverage of Traditional Chinese. |

For unknown-word recognition, also download [CppJieba's `hmm_model.utf8`](https://raw.githubusercontent.com/yanyiwu/cppjieba/v5.6.7/dict/hmm_model.utf8) into that directory.
