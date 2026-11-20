package highlight

import "testing"

func TestConfigurationIconDoesNotChangeDetectedLanguage(t *testing.T) {
	for path, want := range map[string]string{"eslint.config.mjs": "javascript", "eslint.config.cjs": "javascript", "component.jsx": "jsx", "component.tsx": "tsx", "go.mod": "gomod", "main.go": "go", "Makefile": "make", "Dockerfile": "docker", "script.sh": "shellscript", "doc.mdx": "text", "unknown.vl-unknown": "text"} {
		if got := Language(path).ID; got != want {
			t.Fatalf("%s language = %s, want %s", path, got, want)
		}
	}
}

func TestShebangWhenFilenameIsUnknown(t *testing.T) {
	for _, tc := range []struct{ path, source, want string }{
		{"script", "#!/bin/sh\necho hello", "shellscript"},
		{"script", "#! /bin/bash -e\r\necho hello", "shellscript"},
		{"script", "#!/usr/bin/env sh\necho hello", "shellscript"},
		{"script", "#!/usr/bin/env -S zsh -e\necho hello", "shellscript"},
		{"script", "#!/usr/bin/env fish\necho hello", "fish"},
		{"script", "#!/usr/bin/env nu\n", "nushell"},
		{"script", "#!/usr/bin/env -S\nsh", "text"},
		{"script", "#!/usr/bin/false sh\n", "text"},
		{"script", "\n#!/bin/sh\necho hello", "text"},
		{"file.go", "#!/bin/sh\necho hello", "go"},
		{"script", "#!/usr/bin/node\nconsole.log('hello')", "javascript"},
		{"script", "#!/usr/bin/nodejs\n", "javascript"},
		{"script", "#!/usr/bin/env bun\n", "javascript"},
		{"script", "#!/usr/bin/env -S deno run --allow-read\n", "javascript"},
		{"script", "#!/usr/bin/env qjs\n", "javascript"},
		{"main.ts", "#!/usr/bin/env bun\n", "typescript"},
		{"script", "#!/usr/bin/env tsx\n", "typescript"},
		{"script", "#!/usr/bin/env --split-string ts-node --esm\n", "typescript"},
		{"script", "#!/usr/bin/env coffee\n", "coffee"},
		{"script", "#!/usr/bin/python3.13\n", "python"},
		{"script", "#!/usr/bin/env python3\n", "python"},
		{"script", "#!/usr/bin/python3-config\n", "text"},
		{"script", "#!/usr/bin/env -S uv run --script\n", "python"},
		{"script", "#!/usr/bin/env -S uv run bash\n", "text"},
		{"script", "#!/usr/bin/env pypy3.11\n", "python"},
		{"script", "#!/usr/bin/env ruby\n", "ruby"},
		{"script", "#!/usr/bin/env perl\n", "perl"},
		{"script", "#!/usr/bin/env php\n", "php"},
		{"script", "#!/usr/bin/env luajit\n", "lua"},
		{"script", "#!/usr/local/bin/lua5.4\n", "lua"},
		{"script", "#!/usr/bin/perl5.34\n", "perl"},
		{"script", "#!/usr/bin/env php8.4\n", "php"},
		{"script", "#!/usr/bin/env fennel\n", "fennel"},
		{"script", "#!/usr/bin/env gawk\n", "awk"},
		{"script", "#!/usr/bin/env Rscript\n", "r"},
		{"script", "#!/usr/bin/env pwsh\n", "powershell"},
		{"script", "#!/usr/bin/env elixir\n", "elixir"},
		{"script", "#!/usr/bin/env groovy\n", "groovy"},
		{"script", "#!/usr/bin/env julia\n", "julia"},
		{"script", "#!/usr/bin/env escript\n", "erlang"},
		{"script", "#!/usr/bin/env jbang\n", "java"},
		{"script", "#!/usr/bin/env -S kotlinc -script\n", "kotlin"},
		{"script", "#!/usr/bin/env -S scala run\n", "scala"},
		{"script", "#!/usr/bin/env swift\n", "swift"},
		{"script", "#!/usr/bin/env dart\n", "dart"},
		{"script", "#!/usr/bin/env crystal\n", "crystal"},
		{"script", "#!/usr/bin/env rust-script\n", "rust"},
		{"script", "#!/usr/bin/env runghc\n", "haskell"},
		{"script", "#!/usr/bin/env ocaml\n", "ocaml"},
		{"script", "#!/usr/bin/env bb\n", "clojure"},
		{"script", "#!/usr/bin/env hy\n", "hy"},
		{"script", "#!/usr/bin/env sbcl --script\n", "common-lisp"},
		{"script", "#!/usr/bin/env guile\n", "scheme"},
		{"script", "#!/usr/bin/env dotnet-script\n", "csharp"},
		{"script", "#!/usr/bin/env fsharpi\n", "fsharp"},
	} {
		if got := Language(tc.path).WithShebang(tc.source).ID; got != tc.want {
			t.Errorf("%s with %q: language = %s, want %s", tc.path, tc.source, got, tc.want)
		}
	}
}
