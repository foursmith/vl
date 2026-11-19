# Third-party notices

The workbench colors are adapted from VS Code Dark Modern and Light Modern. The terminal icon catalog adapts common file associations and colors from Material Icon Theme, using Nerd Font glyph equivalents rather than distributing its SVG artwork.

## VS Code Dark Modern and Light Modern

Source: https://github.com/microsoft/vscode/blob/main/extensions/theme-defaults/themes/dark_modern.json

```text
MIT License

Copyright (c) 2015 - present Microsoft Corporation

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Material Icon Theme

Source: https://github.com/material-extensions/vscode-material-icon-theme

```text
The MIT License (MIT)
Copyright (c) 2025 Material Extensions

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

Release archives include Go and runtime dependency licenses under `licenses/`.
Nerd Font glyphs require a locally installed font; no font files are bundled.

## textmate-go and bundled TextMate grammars

Syntax highlighting uses `github.com/eugenioenko/textmate-go v0.3.0`, a pure-Go
port of vscode-textmate, with its compressed grammar catalog. The engine's MIT
license and the catalog license and per-grammar notices are preserved under
[third_party/textmate-go](third_party/textmate-go/). Grammar source revisions and
license metadata are recorded in the dependency's `grammars/MANIFEST.md`.

The VS Code theme JSON files embedded under `internal/theme/themes` are covered
by the Microsoft MIT notice above. They come from the
[VS Code 1.135.0 default themes](https://github.com/microsoft/vscode/tree/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/theme-defaults/themes).
