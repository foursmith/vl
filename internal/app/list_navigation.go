package app

// listNavigationKey maps keys that can navigate a list while a text input has
// focus. Lists without a text input can also use navigationState for Vim keys.
func listNavigationKey(k string) string {
	switch {
	case matchesShortcut(k, pickerKeys.Up):
		return "up"
	case matchesShortcut(k, pickerKeys.Down):
		return "down"
	case matchesShortcut(k, pickerKeys.First):
		return "first"
	case matchesShortcut(k, pickerKeys.Last):
		return "last"
	case pageDelta(k, 1) != 0:
		return k
	default:
		return ""
	}
}

// moveListSelection returns the new row and the requested viewport shift.
// The caller owns visibility, section headings, and any preview side effects.
func moveListSelection(action string, selected, rows, height, count int) (index, scroll int, moved bool) {
	index = max(-1, selected)
	switch action {
	case "up":
		index -= count
	case "down":
		index += count
	case "first":
		index = count - 1
	case "last":
		index = rows - 1
	default:
		if height < 1 {
			return selected, 0, false
		}
		delta := pageDelta(action, height) * count
		if delta == 0 {
			return selected, 0, false
		}
		index += delta
		if isHalfPageKey(action) {
			scroll = delta
		}
	}
	return min(rows-1, max(0, index)), scroll, true
}
