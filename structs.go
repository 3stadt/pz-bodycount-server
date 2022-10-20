package main

type configData struct {
	ListenAddress string `yaml:"Listen_Address"`
	Template      string `yaml:"Template_File"`
	ChartTemplate string `yaml:"Template_File_Chart"`
	ModDataDir    string `yaml:"PZ_Mod_Data_Dir"`
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
