package core

import (
	"fmt"
	"os/exec"
)

type HardwareInfo struct {
	OSPlatform       string   `json:"os_platform"`
	OSVersion        string   `json:"os_version"`
	OSRole           string   `json:"os_role"`
	ActivationStatus string   `json:"activation_status"`
	CPU              string   `json:"cpu"`
	GPU              string   `json:"gpu"`
	Motherboard      string   `json:"motherboard"`
	NetworkCard      string   `json:"network_card"`
	MissingDrivers   []string `json:"missing_drivers"`
}

type PackageTargets struct {
	Winget   string `json:"winget,omitempty"`
	Brew     string `json:"brew,omitempty"`
	BrewCask string `json:"brew_cask,omitempty"`
	Apt      string `json:"apt,omitempty"`
	Dnf      string `json:"dnf,omitempty"`
	Pacman   string `json:"pacman,omitempty"`
}

type SoftwareItem struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Category   string         `json:"category"`
	TargetRole string         `json:"target_role,omitempty"`
	BinName    string         `json:"bin_name,omitempty"`
	Targets    PackageTargets `json:"targets"`
	Icon       string         `json:"icon,omitempty"`
	NeedMirror bool           `json:"need_mirror"`
}

type DevEnvExtra struct {
	GitUserName      string `json:"git_user_name"`
	GitUserEmail     string `json:"git_user_email"`
	GitProxyPort     string `json:"git_proxy_port"`
	DockerMirrorURL  string `json:"docker_mirror_url"`
	GoPrivateDomains string `json:"go_private_domains"`
}

type NetworkExtra struct {
	CustomHostname  string `json:"custom_hostname"`
	SSHCustomPort   string `json:"ssh_custom_port"`
	SwapSizeGB      int    `json:"swap_size_gb"`
	DisableHibernat bool   `json:"disable_hibernat"`
	TailscaleKey    string `json:"tailscale_key"`
	TailscaleRoutes string `json:"tailscale_routes"`
	TailscaleExit   bool   `json:"tailscale_exit"`
}

type PatchItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"` // "Security" | "Quality" | "Feature"
}

type PreflightResult struct {
	IsAdmin    bool     `json:"is_admin"`
	FreeDiskGB float64  `json:"free_disk_gb"`
	Warnings   []string `json:"warnings"`
}

type SystemProvider interface {
	CreateRestorePoint(description string) error
	ScanHardware() (*HardwareInfo, error)
	InstallOfficialDrivers(hw *HardwareInfo) error
	ScanSystemPatches() ([]PatchItem, error)
	InstallSystemPatches(securityOnly bool, logFn func(string)) (bool, error)
	OptimizeSystem(options []string) error
	InstallSystemRuntimes() error
	SearchSoftware(keyword string) (string, error)
	InstallSoftware(item SoftwareItem, logFn func(string)) error
	ConfigureDevEnvironment(item SoftwareItem, mirrorURL string) error
	ApplyExtraConfig(dev DevEnvExtra, net NetworkExtra, role string, logFn func(string)) error
	ExportInstalledSoftware() ([]SoftwareItem, error)
	VerifyEnvironment(items []SoftwareItem) []string
	RunPreflightCheck(workDir string) PreflightResult
}

func CommonVerifyEnvironment(items []SoftwareItem) []string {
	var res []string
	for _, item := range items {
		if item.BinName == "" {
			continue
		}
		if path, err := exec.LookPath(item.BinName); err == nil {
			res = append(res, fmt.Sprintf("✅ %-18s | 就绪 (%s)", item.Name, path))
		} else {
			res = append(res, fmt.Sprintf("⚠️ %-18s | 未在 PATH 中检测到 (%s)", item.Name, item.BinName))
		}
	}
	return res
}