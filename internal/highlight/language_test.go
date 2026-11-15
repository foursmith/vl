package highlight

import "testing"

func TestConfigurationIconDoesNotChangeDetectedLanguage(t *testing.T) {
	for path, want := range map[string]string{"eslint.config.mjs": "javascript", "eslint.config.cjs": "javascript", "component.jsx": "jsx", "component.tsx": "tsx", "go.mod": "gomod", "main.go": "go"} {
		if got := Language(path).ID; got != want {
			t.Fatalf("%s language = %s, want %s", path, got, want)
		}
	}
}
