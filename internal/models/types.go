package models

type TaskStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	State   string `json:"state"` // idle, running, success, failed, canceled
	Message string `json:"message"`
}

type FirmwareConfig struct {
	Target       string   `json:"target"` // x86_64, armsr_armv8, rockchip_armv8
	Branch       string   `json:"branch"` // immortalwrt-24.10, openwrt-24.10, etc.
	LanIP        string   `json:"lanIP"`
	RootPassword string   `json:"rootPassword"`
	Theme        string   `json:"theme"`
	Feeds        []string `json:"feeds"`
	Plugins      []string `json:"plugins"`
	GitMirror    string   `json:"gitMirror"` // 可选 GitHub 加速前缀，如 https://ghproxy.net/
}

type FirmwareBundle struct {
	DiyPart1    string `json:"diyPart1"`
	DiyPart2    string `json:"diyPart2"`
	DotConfig   string `json:"dotConfig"`
	WorkflowYml string `json:"workflowYml"`
}

type CatalogOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc,omitempty"`
}

type PluginItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Desc      string `json:"desc"`
	Feed      string `json:"feed"`
	DefaultOn bool   `json:"defaultOn"`
}

type OpenWrtCatalog struct {
	Targets  []CatalogOption `json:"targets"`
	Branches []CatalogOption `json:"branches"`
	Themes   []CatalogOption `json:"themes"`
	Feeds    []CatalogOption `json:"feeds"`
	Plugins  []PluginItem    `json:"plugins"`
}

type AppConfig struct {
	OpenWrt FirmwareConfig `json:"openwrt"`
	PVEHost string         `json:"pveHost"`
	PVEUser string         `json:"pveUser"`
}
