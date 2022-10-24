package main

type configData struct {
	ListenAddress   string `yaml:"Listen_Address"`
	ModDataDir      string `yaml:"PZ_Mod_Data_Dir"`
	FontColor       string `yaml:"Font_Color"`
	ChartFontFamily string `yaml:"Chart_Font_Family"`
	ChartFontSize   string `yaml:"Chart_Font_Size"`
}

type WeaponData struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type pzStats struct {
	Total      int          `json:"total"`
	Categories []WeaponData `json:"categories"`
	Types      []WeaponData `json:"types"`
	ChartData  string
}
