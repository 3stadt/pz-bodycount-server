package main

type configData struct {
	ListenAddress string `yaml:"Listen_Address"`
	Template      string `yaml:"Template_File"`
	ModDataDir    string `yaml:"PZ_Mod_Data_Dir"`
}

type WeaponType struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type WeaponCategory struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type pzStats struct {
	Total      int              `json:"total"`
	Categories []WeaponCategory `json:"categories"`
	Types      []WeaponType     `json:"types"`
}
