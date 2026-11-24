package editor

import "unicode/utf8"

// wordPosition tracks the last rune inspected by a word command. The line
// index resets it when the document changes.
type wordPosition struct {
	runeOffset, byteOffset int
}

// Word commands initialize the line index before they inspect the text.
func (d *Document) wordRuneAt(pos int) rune {
	if pos >= d.runeCount {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(d.Text[d.wordByteOffset(pos):])
	return r
}

func (d *Document) wordByteOffset(pos int) int {
	at := &d.wordPosition
	distance := pos - at.runeOffset
	// Distant jumps use the line index. Nearby accesses decode from the
	// previous position, including across newlines.
	if distance < -64 || distance > 64 {
		*at = wordPosition{runeOffset: pos, byteOffset: d.ByteOffset(pos)}
		return at.byteOffset
	}
	for at.runeOffset < pos {
		_, size := utf8.DecodeRuneInString(d.Text[at.byteOffset:])
		at.byteOffset += size
		at.runeOffset++
	}
	for at.runeOffset > pos {
		_, size := utf8.DecodeLastRuneInString(d.Text[:at.byteOffset])
		at.byteOffset -= size
		at.runeOffset--
	}
	return at.byteOffset
}
