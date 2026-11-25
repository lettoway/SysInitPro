//go:build darwin

package platform

import (
	"SysInitPro/core"
	"fmt"
	"os/exec"
	"strings"
)

type DarwinProvider struct{}

func NewSystemProvider() core.SystemProvider { return &DarwinProvider{} }

func runMacCmd(command string) (string, error) {
	out, err := exec.Command("zsh", "-c", command).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (d *DarwinProvider) RunPreflightCheck(workDir string) core.PreflightResult {
	return core.PreflightResult{IsAdmin: true, FreeDiskGB: 100}
}
func (d *DarwinProvider) CreateRestorePoint(desc string) error {
	_, err := runMacCmd(`tmutil localsnapshot`)
	return err
}
func (d *DarwinProvider) ScanHardware() (*core.HardwareInfo, error) {
	osVer, _ := runMacCmd(`sw_vers -productName && sw_vers -productVersion`)
	cpu, _ := runMacCmd(`sysctl -n machdep.cpu.brand_string`)
	return &core.HardwareInfo{OSPlatform: "darwin", OSVersion: strings.ReplaceAll(osVer, "\n", " "), OSRole: "desktop", ActivationStatus: "macOS 原生", CPU: cpu}, nil
}
func (d *DarwinProvider) InstallOfficialDrivers(hw *core.HardwareInfo) error { return nil }
func (d *DarwinProvider) ScanSystemPatches() ([]core.PatchItem, error) {
	out, _ := runMacCmd(`softwareupdate -l 2>/dev/null | grep "Label:"`)
	var items []core.PatchItem
	for _, line := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			items = append(items, core.PatchItem{ID: "macOS", Title: t, Category: "Security"})
		}
	}
	return items, nil
}
func (d *DarwinProvider) InstallSystemPatches(securityOnly bool, logFn func(string)) (bool, error) {
	_, err := runMacCmd(`softwareupdate -i -a`)
	return false, err
}
func (d *DarwinProvider) OptimizeSystem(options []string) error { return nil }
func (d *DarwinProvider) InstallSystemRuntimes() error {
	_, _ = runMacCmd(`xcode-select --install 2>/dev/null || true`)
	return nil
}
func (d *DarwinProvider) SearchSoftware(keyword string) (string, error) {
	return runMacCmd(fmt.Sprintf(`brew search "%s" | head -n 12`, keyword))
}
func (d *DarwinProvider) InstallSoftware(item core.SoftwareItem, logFn func(string)) error {
	if item.Targets.BrewCask != "" {
		_, err := runMacCmd(`brew install --cask ` + item.Targets.BrewCask)
		return err
	}
	if item.Targets.Brew != "" {
		_, err := runMacCmd(`brew install ` + item.Targets.Brew)
		return err
	}
	return nil
}
func (d *DarwinProvider) ConfigureDevEnvironment(item core.SoftwareItem, mirrorURL string) error { return nil }
func (d *DarwinProvider) ApplyExtraConfig(dev core.DevEnvExtra, net core.NetworkExtra, role string, logFn func(string)) error {
	if dev.GitUserName != "" {
		_, _ = runMacCmd(fmt.Sprintf(`git config --global user.name '%s'`, strings.TrimSpace(dev.GitUserName)))
	}
	return nil
}
func (d *DarwinProvider) ExportInstalledSoftware() ([]core.SoftwareItem, error) { return nil, nil }
func (d *DarwinProvider) VerifyEnvironment(items []core.SoftwareItem) []string {
	return core.CommonVerifyEnvironment(items)
}