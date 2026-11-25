package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHanDictionaryFilePriority(t *testing.T) {
	dir := t.TempDir()
	if dict := loadHanDictionaryFromDir(dir); dict != nil {
		t.Fatal("loaded a dictionary from an empty directory")
	}
	for _, tc := range []struct{ name, contents, word string }{
		{"dict.txt.big", "大版 100 n\n", "大版"},
		{"dict.txt.small", "小版 100 n\n", "小版"},
		{"dict.txt", "invalid dictionary\n", "小版"},
		{"dict.txt", "标准 100 n\n", "标准"},
	} {
		if err := os.WriteFile(filepath.Join(dir, tc.name), []byte(tc.contents), 0600); err != nil {
			t.Fatal(err)
		}
		dict := loadHanDictionaryFromDir(dir)
		if dict == nil {
			t.Fatalf("no dictionary after writing %s", tc.name)
		}
		if _, ok := dict.scores[tc.word]; !ok || len(dict.scores) != 1 {
			t.Fatalf("after writing %s: got %v, want %q", tc.name, dict.scores, tc.word)
		}
	}
}

func testHanDictionary(t *testing.T) func() *HanDictionary {
	t.Helper()
	dir := t.TempDir()
	const words = `中文 1755 nz
分词 19 n
分詞 19 n
测试 2444 v
測試 2444 v
研究 35029 v
研究生 1816 n
生命 6986 n
起源 1504 n
正在 5041 d
来到 16552 v
网易 619 nz
大厦 1662 n
`
	const model = `# B E M S
-0.26 -3.14e100 -3.14e100 -1.46

# Transition probabilities
-3.14e100 -0.51 -0.91 -3.14e100
-0.59 -3.14e100 -3.14e100 -0.80
-3.14e100 -0.33 -1.26 -3.14e100
-0.72 -3.14e100 -3.14e100 -0.66

# Emission probabilities
杭:-1
研:-1
杭:-12
杭:-10,研:-10
`
	dictPath := filepath.Join(dir, "dict.txt.big")
	if err := os.WriteFile(dictPath, []byte(words), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hmm_model.utf8"), []byte(model), 0600); err != nil {
		t.Fatal(err)
	}
	dict := loadHanDictionary(dictPath)
	if dict == nil || dict.unknownWords() == nil {
		t.Fatal("could not load the Jieba text fixtures")
	}
	return func() *HanDictionary { return dict }
}

func TestVimChineseWordBoundaries(t *testing.T) {
	dict := testHanDictionary(t)
	for _, tt := range []struct {
		text, keys string
		start, end int
	}{
		{"中文分词测试", "2w", 0, 4},
		{"中文分詞測試", "b", 4, 2},
		{"研究生命起源", "e", 2, 3},
		{"中文分词测试", "e", 0, 1},
		{"中文𠀀𠀁测试", "w", 2, 3},
		{"中\ufe0f文分词测试", "w", 0, 2},
		{"foo_bar中文分词测试。end", "w", 7, 9},
		{"他来到了网易杭研大厦", "w", 6, 8},
		{"他来到了网易杭研大厦", "b", 8, 6},
		{"他来到了网易杭研大厦", "e", 6, 7},
		{"𠀀杭研𠀁", "w", 1, 3},
	} {
		d := &Document{Buffer: Buffer{Text: tt.text, Anchor: -1}}
		d.SetPosition(0, tt.start)
		v := &Vim{HanDictionary: dict}
		for _, key := range tt.keys {
			vimKeys(d, v, string(key))
		}
		if d.Cursor != tt.end {
			t.Errorf("%q %s from %d: cursor %d, want %d", tt.text, tt.keys, tt.start, d.Cursor, tt.end)
		}
	}
}

func TestVimChineseWordObjectUsesHMM(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "他来到了网易杭研大厦", Anchor: -1}}
	d.SetPosition(0, 7)
	copied := vimKeys(d, &Vim{HanDictionary: testHanDictionary(t)}, "d", "i", "w")
	if d.Text != "他来到了网易大厦" || copied != "杭研" {
		t.Fatalf("text %q, register %q", d.Text, copied)
	}
}

func TestVimChineseWordEditsUseCurrentText(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "中文分词测试", Anchor: -1}}
	v := &Vim{HanDictionary: testHanDictionary(t)}
	vimKeys(d, v, "w", "d", "w")
	if d.Text != "中文测试" {
		t.Fatal(d.Text)
	}
	vimKeys(d, v, "u")
	d.SetPosition(0, 2)
	vimKeys(d, v, "c", "w", "X", "esc")
	if d.Text != "中文X测试" {
		t.Fatal(d.Text)
	}
	d.Edit("我正在测试")
	d.SetPosition(0, 0)
	vimKeys(d, v, "w")
	if d.Cursor != 1 {
		t.Fatalf("word cache after editing: cursor %d", d.Cursor)
	}
	if got := vimKeys(d, v, "y", "i", "W"); got != d.Text {
		t.Fatalf("yiW = %q, want %q", got, d.Text)
	}
}

func TestVimWordLookupsAfterJumpAndEdit(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: strings.Repeat("中文", 80) + "\n中文分词测试\n𠀀𠀁结果", Anchor: -1}}
	v := &Vim{HanDictionary: testHanDictionary(t)}
	d.SetPosition(1, 0)
	vimKeys(d, v, "3", "w", "b")
	if row, col := d.Position(); row != 1 || col != 4 {
		t.Fatalf("cursor after crossing a newline twice: %d:%d", row, col)
	}
	if word := vimKeys(d, v, "y", "i", "w"); word != "测试" {
		t.Fatalf("yanked %q, want 测试", word)
	}
	// Keep the second line's rune offset while changing its byte offset.
	d.Edit(strings.Repeat("a", 160) + "\n研究生命起源\n𠀀𠀁结果")
	d.SetPosition(1, 4)
	vimKeys(d, v, "b")
	if word := vimKeys(d, v, "y", "i", "w"); word != "生命" {
		t.Fatalf("after editing, yanked %q, want 生命", word)
	}
}

func TestVimWordMotionWithMissingDictionary(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "中文分词测试 next", Anchor: -1}}
	vimKeys(d, &Vim{HanDictionary: func() *HanDictionary { return nil }}, "w")
	if d.Cursor != 7 {
		t.Fatalf("word motion with missing dictionary: cursor %d, want 7", d.Cursor)
	}
}
