package editor

import (
	"crypto/sha256"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/foursmith/vl/internal/config"
)

// Undo can move the previously saved text onto the redo stack.
const maxUndoHistoryBytes = maxUndoBytes + MaxFileSize

const (
	undoFileMagic       = "VLUNDO\x00\x02"
	undoFileHeaderSize  = len(undoFileMagic) + sha256.Size
	undoStackCountsSize = 2
	undoRecordSize      = 5*4 + 1
	maxUndoPayloadBytes = maxUndoHistoryBytes + undoRecordSize*maxUndoSteps + undoStackCountsSize + crc32.Size
)

type undoRecord struct {
	Start, Remove, InsertBytes uint32
	Cursor, Anchor             int32
	SelectionKind              uint8
}

// Each stack starts at the saved document, with the nearest change first.
func appendUndoStack(data []byte, base string, states []editState) []byte {
	for i := len(states) - 1; i >= 0; i-- {
		state := states[i]
		start, end, nextEnd := undoChangedRange(base, state.text)
		record := undoRecord{
			Start: uint32(start), Remove: uint32(end - start), InsertBytes: uint32(nextEnd - start),
			Cursor: int32(state.cursor), Anchor: int32(state.anchor), SelectionKind: state.selectionKind,
		}
		data, _ = binary.Append(data, binary.LittleEndian, record)
		data = append(data, state.text[start:nextEnd]...)
		base = state.text
	}
	return data
}

func undoChangedRange(before, after string) (start, end, nextEnd int) {
	limit := min(len(before), len(after))
	for start < limit && before[start] == after[start] {
		start++
	}
	// Avoid splitting a rune with an identical leading byte.
	for start > 0 && start < len(before) && !utf8.RuneStart(before[start]) {
		start--
	}
	end, nextEnd = len(before), len(after)
	for end > start && nextEnd > start && before[end-1] == after[nextEnd-1] {
		end--
		nextEnd--
	}
	// Equal trailing bytes may also be part of different runes.
	for end < len(before) && !utf8.RuneStart(before[end]) {
		end++
		nextEnd++
	}
	return start, end, nextEnd
}

func readUndoStack(data *[]byte, base string, count int, budget *int) ([]editState, bool) {
	if count == 0 {
		return nil, true
	}
	states := make([]editState, count)
	for i := range states {
		var record undoRecord
		n, err := binary.Decode(*data, binary.LittleEndian, &record)
		if err != nil || record.Start > uint32(len(base)) ||
			record.Remove > uint32(len(base))-record.Start ||
			record.InsertBytes > uint32(len(*data)-n) ||
			record.Cursor < 0 || record.Anchor < -1 || record.SelectionKind > 2 {
			return nil, false
		}
		start, end := int(record.Start), int(record.Start+record.Remove)
		if (start < len(base) && !utf8.RuneStart(base[start])) ||
			(end < len(base) && !utf8.RuneStart(base[end])) {
			return nil, false
		}
		insertEnd := n + int(record.InsertBytes)
		insert := (*data)[n:insertEnd]
		size := len(base) - int(record.Remove) + len(insert)
		if size > *budget || !utf8.Valid(insert) {
			return nil, false
		}
		*budget -= size
		base = base[:start] + string(insert) + base[end:]
		*data = (*data)[insertEnd:]
		states[count-1-i] = editState{
			text: base, cursor: int(record.Cursor), anchor: int(record.Anchor), selectionKind: record.SelectionKind,
		}
	}
	return states, true
}

func undoFileHeader(disk []byte) [undoFileHeaderSize]byte {
	var header [undoFileHeaderSize]byte
	copy(header[:], undoFileMagic)
	hash := sha256.Sum256(disk)
	copy(header[len(undoFileMagic):], hash[:])
	return header
}

func (d *Document) loadUndoHistory() {
	// Leave file previews and watcher reads on the normal file-open path.
	if d.undoHistoryLoaded {
		return
	}
	d.undoHistoryLoaded = true
	if d.Path == "" {
		return
	}
	path, err := config.UndoHistoryPath(d.Path)
	if err != nil {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	expected := undoFileHeader(d.saved)
	var header [undoFileHeaderSize]byte
	if _, err := io.ReadFull(file, header[:]); err != nil || header != expected {
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxUndoPayloadBytes+1))
	if err != nil || len(data) < undoStackCountsSize+crc32.Size || len(data) > maxUndoPayloadBytes {
		return
	}
	payload, checksum := data[:len(data)-crc32.Size], data[len(data)-crc32.Size:]
	undoCount, redoCount := int(payload[0]), int(payload[1])
	if undoCount+redoCount > maxUndoSteps || crc32.ChecksumIEEE(payload) != binary.LittleEndian.Uint32(checksum) {
		return
	}
	payload = payload[undoStackCountsSize:]
	budget := maxUndoHistoryBytes
	undo, ok := readUndoStack(&payload, d.savedText, undoCount, &budget)
	if !ok {
		return
	}
	redo, ok := readUndoStack(&payload, d.savedText, redoCount, &budget)
	if ok && len(payload) == 0 {
		d.undo, d.redo = undo, redo
	}
}

func (d *Document) saveUndoHistory() {
	undo, redo := d.undo, d.redo
	count := len(undo) + len(redo)
	if count == 0 || count > maxUndoSteps || !utf8.ValidString(d.savedText) {
		return
	}
	path, err := config.UndoHistoryPath(d.Path)
	if err != nil {
		return
	}
	// Apply the same reconstructed byte limit as the reader.
	size := 0
	for _, stack := range [][]editState{undo, redo} {
		for _, state := range stack {
			size += len(state.text)
			if size > maxUndoHistoryBytes || !utf8.ValidString(state.text) {
				return
			}
		}
	}
	header := undoFileHeader(d.saved)
	data := append(header[:], byte(len(undo)), byte(len(redo)))
	data = appendUndoStack(data, d.savedText, undo)
	data = appendUndoStack(data, d.savedText, redo)
	data = binary.LittleEndian.AppendUint32(data, crc32.ChecksumIEEE(data[len(header):]))
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0700) != nil {
		return
	}
	file, err := os.CreateTemp(dir, ".undo-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	// History is optional. If writing fails, keep the previous complete record.
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		_ = os.Rename(file.Name(), path)
	}
}
