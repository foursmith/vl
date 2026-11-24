package app

import (
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

type textInput struct {
	textinput.Model
	anchor       int
	viewportLeft int // Byte offset kept stable when the mouse moves the caret.
	undo, redo   []inputSnapshot
	revision     uint64
}

type inputSnapshot struct {
	value            string
	position, anchor int
}

func (i *textInput) SetValue(value string) {
	i.Model.SetValue(value)
	i.viewportLeft = 0
	i.anchor = -1
	i.undo, i.redo = nil, nil
	i.revision++
}

func (i *textInput) Blur() {
	i.Model.Blur()
	i.anchor = -1
	i.revision++
}

func (i textInput) snapshot() inputSnapshot {
	return inputSnapshot{i.Value(), i.Position(), i.anchor}
}

func (i *textInput) restore(from, to *[]inputSnapshot) {
	if len(*from) == 0 {
		return
	}
	*to = append(*to, i.snapshot())
	last := len(*from) - 1
	saved := (*from)[last]
	(*from)[last] = inputSnapshot{}
	*from = (*from)[:last]
	i.Model.SetValue(saved.value)
	i.viewportLeft = 0
	i.SetCursor(saved.position)
	i.anchor = saved.anchor
	i.revision++
}

func (i textInput) selection() (start, end int) {
	if i.anchor < 0 {
		return i.Position(), i.Position()
	}
	return min(i.anchor, i.Position()), max(i.anchor, i.Position())
}

type inputCutMsg struct{}

func (i *textInput) edit(action textEditAction) tea.Cmd {
	switch action {
	case textSelectAll:
		i.anchor = 0
		i.CursorEnd()
		i.revision++
	case textCopy, textCut:
		start, end := i.selection()
		if start == end {
			return nil
		}
		var apply clipboardApply
		if action == textCut {
			i.revision++
			apply = i.clipboardTarget(true)
		}
		return writeClipboard(string([]rune(i.Value())[start:end]), apply)
	case textPaste:
		i.revision++
		return readClipboard(i.clipboardTarget(false))
	case textUndo:
		i.restore(&i.undo, &i.redo)
	case textRedo:
		i.restore(&i.redo, &i.undo)
	}
	return nil
}

func (i *textInput) update(msg tea.Msg) tea.Cmd {
	if !i.Focused() {
		return nil
	}
	if action, ok := msg.(textEditAction); ok {
		return i.edit(action)
	}
	before := i.snapshot()
	start, end := i.selection()
	replace := false
	switch msgKey := msg.(type) {
	case tea.KeyPressMsg:
		if i.extendSelection(msgKey) {
			i.revision++
			return nil
		}
		if start == end && key.Matches(msgKey, i.KeyMap.DeleteWordForward) &&
			before.position == utf8.RuneCountInString(before.value)-1 {
			// Bubbles' forward word deletion reads past the last rune here.
			msg = tea.KeyPressMsg{Code: tea.KeyDelete}
		}
		if start != end {
			switch {
			case key.Matches(msgKey, i.KeyMap.DeleteCharacterBackward, i.KeyMap.DeleteCharacterForward,
				i.KeyMap.DeleteWordBackward, i.KeyMap.DeleteWordForward):
				replace, msg = true, nil
			case key.Matches(msgKey, i.KeyMap.CharacterBackward):
				i.SetCursor(start)
				i.anchor = -1
				i.revision++
				return nil
			case key.Matches(msgKey, i.KeyMap.CharacterForward):
				i.SetCursor(end)
				i.anchor = -1
				i.revision++
				return nil
			case key.Matches(msgKey, i.KeyMap.LineStart, i.KeyMap.LineEnd, i.KeyMap.WordBackward, i.KeyMap.WordForward):
				i.anchor = -1
			default:
				replace = msgKey.Text != ""
			}
		}
	case tea.PasteMsg:
		replace = msgKey.Content != ""
	case inputCutMsg:
		replace, msg = true, nil
	}

	if replace && start != end {
		runes := []rune(before.value)
		i.Model.SetValue(string(runes[:start]) + string(runes[end:]))
		i.SetCursor(start)
		i.anchor = -1
	}
	var cmd tea.Cmd
	if msg != nil {
		i.Model, cmd = i.Model.Update(msg)
	}
	after := i.snapshot()
	if after.value != before.value {
		i.viewportLeft = 0
		const historyLimit = 100
		if len(i.undo) == historyLimit {
			copy(i.undo, i.undo[1:])
			i.undo = i.undo[:historyLimit-1]
		}
		i.undo = append(i.undo, before)
		i.redo = nil
		i.anchor = -1
	} else if after.position != before.position {
		i.anchor = -1
	}
	if after != before || i.anchor != before.anchor {
		i.revision++
	}
	return cmd
}

func (i *textInput) extendSelection(msg tea.KeyPressMsg) bool {
	if !msg.Mod.Contains(tea.ModShift) {
		return false
	}
	move := msg
	move.Mod &^= tea.ModShift
	start := i.Position()
	direction := move.String()
	switch {
	case direction == leftArrow || direction == rightArrow:
		position := 0
		iter := graphemes.FromString(i.Value())
		for iter.Next() {
			next := position + utf8.RuneCountInString(iter.Value())
			if direction == leftArrow && next >= start {
				i.SetCursor(position)
				break
			}
			if direction == rightArrow && next > start {
				i.SetCursor(next)
				break
			}
			position = next
		}
	case key.Matches(move, i.KeyMap.LineStart, i.KeyMap.LineEnd, i.KeyMap.WordBackward, i.KeyMap.WordForward):
		i.Model, _ = i.Model.Update(move)
	default:
		return false
	}
	if i.anchor < 0 {
		i.anchor = start
	}
	return true
}
