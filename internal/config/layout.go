package config

// Layout stores user preferences independently of the current terminal size.
type Layout struct {
	SidebarWidth int  `json:"sidebarWidth"`
	SidebarRight bool `json:"sidebarRight"`
}

func LayoutPath() (string, error) { return configPath("layout.json") }

func LoadLayout(path string) (Layout, error) {
	var layout Layout
	err := loadJSON(path, &layout)
	return layout, err
}

func SaveLayout(path string, layout Layout) error { return saveJSON(path, layout) }
