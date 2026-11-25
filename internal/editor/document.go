package editor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxFileSize = 4 << 20

// Document adds file encoding, save state, and persistent undo to a text buffer.
type Document struct {
	Buffer
	Path                string
	saved               []byte
	savedText           string
	indentation         string
	savedCRLF, savedBOM bool
	CRLF, BOM           bool
}

func Open(path string) (*Document, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("only regular text files can be edited")
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("basic editor limit is %d MiB", MaxFileSize>>20)
	}
	f, err := os.Open(real)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("basic editor limit is %d MiB", MaxFileSize>>20)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, errors.New("binary or non-UTF-8 file")
	}
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if bytes.Contains(data, []byte("\r\n")) && bytes.Count(data, []byte("\n")) != bytes.Count(data, []byte("\r\n")) {
		return nil, errors.New("mixed LF/CRLF line endings; normalize the file before editing")
	}
	if bytes.ContainsRune(normalized, '\r') {
		return nil, errors.New("legacy CR line endings are not supported by the basic editor")
	}
	text := strings.TrimPrefix(string(normalized), "\xef\xbb\xbf")
	d := &Document{
		Buffer: Buffer{Text: text, Anchor: -1},
		Path:   real,
		saved:  data,
		CRLF:   bytes.Contains(data, []byte("\r\n")),
		BOM:    bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}),
	}
	// Defer loading history until the first edit, undo, or save.
	d.readHistory = func() ([]editState, []editState) {
		return readUndoHistory(real, data, text)
	}
	d.savedText, d.savedCRLF, d.savedBOM = d.Text, d.CRLF, d.BOM
	return d, nil
}

// EncodedSize reports the on-disk size for text with this document's file format.
func (d *Document) EncodedSize(text string) int {
	size := len(text)
	if d.CRLF {
		size += strings.Count(text, "\n")
	}
	if d.BOM {
		size += len("\xef\xbb\xbf")
	}
	return size
}

func (d *Document) encode(text string) []byte {
	if d.CRLF {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if d.BOM {
		text = "\xef\xbb\xbf" + text
	}
	return []byte(text)
}

// SameDisk compares the last disk snapshots without changing save conflict guards.
func (d *Document) SameDisk(other *Document) bool { return bytes.Equal(d.saved, other.saved) }

func (d *Document) Dirty() bool {
	return !d.ReadOnly && (d.Text != d.savedText || d.CRLF != d.savedCRLF || d.BOM != d.savedBOM)
}

func (d *Document) Save() error {
	data := d.encode(d.Text)
	if err := d.writeFile(data); err != nil {
		return err
	}
	d.loadUndoHistory()
	d.markSaved(data)
	d.saveUndoHistory()
	return nil
}

// SaveWorkspaceEdit makes the replacement the buffer's new undo baseline.
func (d *Document) SaveWorkspaceEdit(text string) error {
	data := d.encode(text)
	if err := d.writeFile(data); err != nil {
		return err
	}
	if text != d.Text {
		// Existing snapshots would also undo the workspace replacement.
		d.EndEditGroup()
		d.undo, d.redo = nil, nil
		d.readHistory = nil
	}
	d.Text = text
	d.markSaved(data)
	return nil
}

func (d *Document) markSaved(data []byte) {
	d.saved = data
	d.savedText, d.savedCRLF, d.savedBOM = d.Text, d.CRLF, d.BOM
}

func (d *Document) writeFile(data []byte) error {
	if d.ReadOnly {
		return errors.New("file is read-only")
	}
	current, err := os.ReadFile(d.Path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, d.saved) {
		return errors.New("file changed on disk; reopen it before saving")
	}
	info, err := os.Stat(d.Path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0222 == 0 {
		return errors.New("file is read-only")
	}
	tmp, err := os.CreateTemp(filepath.Dir(d.Path), ".vl-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(info.Mode().Perm()); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Recheck after writing the temporary file so external edits are not silently overwritten.
	current, err = os.ReadFile(d.Path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, d.saved) {
		return errors.New("file changed on disk while saving")
	}
	return os.Rename(tmp.Name(), d.Path)
}
