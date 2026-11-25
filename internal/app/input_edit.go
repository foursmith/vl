package app

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/foursmith/vl/internal/editor"
)

const inputRuneLimit = 2000

type textInput struct {
	Prompt, Placeholder string
	buffer              editor.Buffer
	focused             bool
	styles              inputStyles
	viewportLeft        int // Byte offset kept stable when the mouse moves the caret.
	revision            uint64
}

func (i *textInput) Value() string { return i.buffer.Text }
func (i *textInput) Position() int { return i.buffer.Cursor }

func (i *textInput) SetValue(value string) {
	value = singleLineInput(value, inputRuneLimit)
	i.buffer = editor.Buffer{Text: value, Cursor: utf8.RuneCountInString(value), Anchor: -1}
	i.viewportLeft = 0
	i.revision++
}

func (i *textInput) Focus()        { i.focused = true }
func (i *textInput) Focused() bool { return i.focused }

func (i *textInput) Blur() {
	i.focused = false
	i.buffer.ClearSelection()
	i.revision++
}

type inputCutMsg struct{}

func (i *textInput) edit(action commandID) tea.Cmd {
	switch action {
	case commandSelectAll:
		i.buffer.SelectAll()
	case commandCopy, commandCut:
		selected := i.buffer.SelectedText()
		var apply clipboardApply
		if action == commandCut && selected != "" {
			i.revision++
			apply = i.clipboardTarget(true)
		}
		return writeClipboard(selected, apply)
	case commandPaste:
		i.revision++
		return readClipboard(i.clipboardTarget(false))
	case commandDeleteToLineStart, commandDeleteToLineEnd:
		i.buffer.DeleteToLineBoundary(action == commandDeleteToLineStart)
	case commandUndo:
		i.buffer.Undo()
	case commandRedo:
		i.buffer.Redo()
	}
	return nil
}

func (i *textInput) update(msg tea.Msg, dictionary func() *editor.HanDictionary) tea.Cmd {
	if !i.Focused() {
		return nil
	}
	before, cursor, anchor := i.buffer.Text, i.buffer.Cursor, i.buffer.Anchor
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case textEditAction:
		cmd = i.edit(msg.command)
	case tea.KeyPressMsg:
		if msg.Text != "" {
			i.insert(msg.Text)
		} else {
			editBufferKey(&i.buffer, msg.String(), dictionary)
		}
	case tea.PasteMsg:
		i.insert(msg.Content)
	case inputCutMsg:
		i.buffer.Insert("")
	}

	textChanged := before != i.buffer.Text
	if textChanged {
		i.viewportLeft = 0
	}
	if textChanged || cursor != i.buffer.Cursor || anchor != i.buffer.Anchor {
		i.revision++
	}
	return cmd
}

func (i *textInput) insert(text string) {
	if text == "" {
		return
	}
	start, end := i.buffer.Selection()
	available := inputRuneLimit - (i.buffer.LineRuneCount(0) - (end - start))
	if text = singleLineInput(text, available); text != "" {
		i.buffer.Insert(text)
	}
}

func singleLineInput(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	text = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, text)
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	iter := graphemes.FromString(text)
	for iter.Next() {
		limit -= utf8.RuneCountInString(iter.Value())
		if limit < 0 {
			return text[:iter.Start()]
		}
	}
	return text
}
