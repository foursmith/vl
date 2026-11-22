package app

// UI controls use Codicons from Nerd Fonts. Material file associations live in
// fileicon; punctuation, borders and diff signs are text rather than controls.
type uiSymbol int

const (
	symbolExplorer uiSymbol = iota
	symbolBranch
	symbolHistory
	symbolRefresh
	symbolOpen
	symbolDiscard
	symbolAdd
	symbolRemove
	symbolClose
	symbolLeft
	symbolRight
	symbolDown
	symbolSync
	symbolGitHub
	symbolCloud
	symbolTag
	symbolCheck
	symbolUp
	symbolArrowDown
	symbolDiff
	symbolPublish
	symbolWarning
	symbolDot
	symbolRepository
	symbolSearch
	symbolDiffSplit
	symbolDiffUnified
	symbolSearchDetails
	symbolOpenTabs
	symbolIgnoreRules
	symbolPullRequest
	symbolFilter
	symbolFold
)

var uiSymbols = [...]struct{ plain, nerd string }{
	symbolExplorer:  {"E", "\ueaf0"},
	symbolBranch:    {"G", "\uea68"},
	symbolHistory:   {"H", "\uea82"},
	symbolRefresh:   {"R", "\ueb37"},
	symbolOpen:      {">", "\uea94"},
	symbolDiscard:   {"~", "\ueae2"},
	symbolAdd:       {"+", "\uea60"},
	symbolRemove:    {"-", "\ueb3b"},
	symbolClose:     {"x", "\uea76"},
	symbolLeft:      {"<", "\ueab5"},
	symbolRight:     {">", "\ueab6"},
	symbolDown:      {"v", "\ueab4"},
	symbolSync:      {"R", "\uea77"},
	symbolGitHub:    {"GitHub", "\uea84"},
	symbolCloud:     {"@", "\uebaa"},
	symbolTag:       {"#", "\uea66"},
	symbolCheck:     {"*", "\ueab2"},
	symbolUp:        {"^", "\ueaa1"},
	symbolArrowDown: {"v", "\uea9a"},
	symbolDiff:      {"D", "\ueae1"},
	symbolPublish:   {"^", "\ueac3"},
	symbolWarning:   {"!", "\uea6c"},
	symbolDot:       {"*", "\uea71"},

	symbolSearch:        {"S", "\uea6d"},
	symbolRepository:    {"R", "\uea62"},
	symbolDiffSplit:     {"[|]", "\ueb56"},
	symbolDiffUnified:   {"[=]", "\ueb57"},
	symbolSearchDetails: {"...", "\uea7c"},
	symbolOpenTabs:      {"Tab", "\ueb7f"},
	symbolIgnoreRules:   {"Ignore rules", "\ueadd"},
	symbolPullRequest:   {"PR", "\uea64"},
	symbolFilter:        {"F", "\ueaf1"},
	symbolFold:          {"x", "\ueaf5"},
}

func (m *Model) symbol(name uiSymbol) string {
	if m.nerd {
		return uiSymbols[name].nerd
	}
	return uiSymbols[name].plain
}

func (m *Model) treeChevron(expanded bool) string {
	if expanded {
		return m.symbol(symbolDown)
	}
	return m.symbol(symbolRight)
}
