package main

import (
	"SysInitPro/config"
	"SysInitPro/core"
	"SysInitPro/network"
	"SysInitPro/openwrt"
	"SysInitPro/platform"
	"SysInitPro/pve"
	"SysInitPro/sshx"
	"SysInitPro/translator"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	stdRuntime "runtime"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const defaultCatalogFile = "sysinit_catalog_v9.json"

type App struct {
	ctx      context.Context
	provider core.SystemProvider
	running  atomic.Bool
}

func NewApp() *App { return &App{provider: platform.NewSystemProvider()} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) pushLog(percent int, message string) {
	if msg := strings.TrimSpace(message); msg != "" {
		if a.ctx == nil {
			fmt.Printf("[%3d%%] %s\n", percent, msg)
			return
		}
		runtime.EventsEmit(a.ctx, "task:progress", map[string]interface{}{"percent": percent, "message": msg})
	}
}

func (a *App) pushOpenWrtLog(message string) {
	if msg := strings.TrimSpace(message); msg != "" && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "openwrt:log", msg)
	}
}

// ================= 基础硬件、预检、UI 状态持久化与系统补丁 =================
func (a *App) ScanHardware() (*core.HardwareInfo, error) { return a.provider.ScanHardware() }
func (a *App) RunPreflightCheck(workDir string) core.PreflightResult { return a.provider.RunPreflightCheck(workDir) }
func (a *App) SaveUIState(state map[string]string) error { return config.SaveUIState(state) }
func (a *App) LoadUIState() map[string]string { return config.LoadUIState() }
func (a *App) GetFastestMirror() network.MirrorNode { return network.BenchmarkMirrors() }
func (a *App) GetAvailableMirrors() []network.MirrorNode { return network.GetAllMirrors() }

func (a *App) ScanSystemPatches() ([]core.PatchItem, error) { return a.provider.ScanSystemPatches() }
func (a *App) InstallSystemPatchesNow(securityOnly bool) (string, error) {
	reboot, err := a.provider.InstallSystemPatches(securityOnly, func(s string) { a.pushLog(-1, s) })
	if err != nil {
		return "", err
	}
	if reboot {
		return "✅ 补丁安装完成！⚠️ 检测到系统待重启标记，请在保存工作后手动重启电脑。", nil
	}
	return "✅ 补丁安装完成，无需重启！", nil
}

func (a *App) ActivateWindowsKey(key string) error {
	if wp, ok := a.provider.(interface{ ActivateWithProductKey(string) error }); ok {
		return wp.ActivateWithProductKey(key)
	}
	return fmt.Errorf("当前系统不支持产品密钥激活")
}

// ================= 逆向导入、环境体检与 Gist 云同步 =================
func (a *App) ImportLocalEnvironment() ([]core.SoftwareItem, error) { return a.provider.ExportInstalledSoftware() }
func (a *App) VerifyInstalledEnv(items []core.SoftwareItem) []string { return a.provider.VerifyEnvironment(items) }

func (a *App) SyncToGitHubGist(token, gistID string, profile config.SetupProfile, catalog []core.SoftwareItem) (*network.GistSyncResult, error) {
	profile.NetExtra.TailscaleKey = ""
	pBytes, _ := json.MarshalIndent(profile, "", "  ")
	cBytes, _ := json.MarshalIndent(catalog, "", "  ")
	return network.SyncToGist(token, gistID, string(pBytes), string(cBytes))
}

func (a *App) RestoreFromGitHubGist(token, gistID string) (*config.SetupProfile, error) {
	pJSON, cJSON, err := network.RestoreFromGist(token, gistID)
	if err != nil {
		return nil, err
	}
	if cJSON != "" {
		var list []core.SoftwareItem
		if json.Unmarshal([]byte(cJSON), &list) == nil && len(list) > 0 {
			_ = a.SaveSoftwareCatalog(list)
		}
	}
	var prof config.SetupProfile
	if pJSON != "" {
		_ = json.Unmarshal([]byte(pJSON), &prof)
	}
	return &prof, nil
}

// ================= 统一 SSH 资产簿、VPS 加固、Sing-box 与 PVE 编排 =================
func (a *App) ListSSHHosts() []sshx.LocalSSHHost { return sshx.ListLocalSSHHosts() }
func (a *App) ParseSingboxURI(uri string) (*translator.SingboxFormConfig, error) { return translator.ParseSingboxURI(uri) }
func (a *App) PreviewSingboxConfig(cfg translator.SingboxFormConfig) (string, error) { return translator.GenerateSingboxJSON(cfg) }
func (a *App) DeploySingboxRemote(t sshx.SSHTarget, sbCfg translator.SingboxFormConfig) (string, error) { return translator.DeploySingboxToVPS(t, sbCfg) }
func (a *App) RunVPSRemoteCommand(t sshx.SSHTarget, cmdStr string) (string, error) { return sshx.RunCommand(t, cmdStr) }
func (a *App) PreviewVPSSecurityScript(cfg translator.VPSSecurityConfig) string {
	if pub, _, err := translator.EnsureLocalSSHKeyAndConfig(translator.VPSSecurityConfig{}); err == nil && cfg.PublicKey == "" {
		cfg.PublicKey = pub
	}
	return translator.GenerateVPSSecurityScript(cfg)
}
func (a *App) ExecuteVPSSecurityHardening(cfg translator.VPSSecurityConfig) (string, error) { return translator.RunVPSSSHProvision(cfg) }

func (a *App) PVEUpgradeHost(cfg pve.PVEClusterConfig) (string, error) { return pve.UpgradePVEHost(cfg) }
func (a *App) PVEConfigureBootOrder(cfg pve.PVEClusterConfig) (string, error) { return pve.ConfigureBootOrder(cfg) }
func (a *App) PVESafeUpgradeVMs(cfg pve.PVEClusterConfig) (string, error) { return pve.SafeUpgradeVMs(cfg) }
func (a *App) PVEDeploySubBoost(zorinSSH sshx.SSHTarget) (string, error) { return pve.DeploySubBoostToZorin(zorinSSH) }
func (a *App) PVEUpgradeOpenWrtFirmwareDialog(cfg pve.PVEClusterConfig) (string, error) {
	fwPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择要推送升级的 OpenWrt 固件 (.img.gz / .bin)"})
	if err != nil || fwPath == "" {
		return "", fmt.Errorf("未选择固件文件")
	}
	return pve.UpgradeOpenWrtFirmware(cfg, fwPath)
}

// ================= 剧本翻译、OpenWrt 编译与软件目录 =================
func (a *App) PreviewPlaybookScript(p config.SetupProfile) string {
	if p.TargetOS == "windows" || (p.TargetOS == "" && stdRuntime.GOOS == "windows") {
		return translator.TranslateToPowerShell(p)
	}
	return translator.TranslateToUnixShell(p)
}
func (a *App) ExportPlaybookScript(p config.SetupProfile) (string, error) {
	name := "sysinit_playbook.sh"
	if p.TargetOS == "windows" || (p.TargetOS == "" && stdRuntime.GOOS == "windows") {
		name = "sysinit_playbook.ps1"
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "导出原生初始化剧本", DefaultFilename: name})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(a.PreviewPlaybookScript(p)), 0755)
}
func (a *App) ExportSingboxConfig(cfg translator.SingboxFormConfig) (string, error) {
	raw, err := translator.GenerateSingboxJSON(cfg)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "导出 Sing-box 配置", DefaultFilename: "config.json"})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(raw), 0644)
}

func (a *App) PreviewOpenWrtScripts(cfg openwrt.FirmwareConfig) openwrt.GeneratedBundle { return openwrt.BuildFirmwareBundle(cfg) }
func (a *App) ExportOpenWrtBundle(cfg openwrt.FirmwareConfig) (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择导出 OpenWrt 编译包目录"})
	if err != nil || dir == "" {
		return "", err
	}
	b := openwrt.BuildFirmwareBundle(cfg)
	_ = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "diy-part1.sh"), []byte(b.DiyPart1), 0755)
	_ = os.WriteFile(filepath.Join(dir, "diy-part2.sh"), []byte(b.DiyPart2), 0755)
	_ = os.WriteFile(filepath.Join(dir, "diff.config"), []byte(b.DotConfig), 0644)
	_ = os.WriteFile(filepath.Join(dir, "build-local-docker.sh"), []byte(b.DockerScript), 0755)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "build-openwrt.yml"), []byte(b.WorkflowYml), 0644)
	return dir, nil
}
func (a *App) StartOpenWrtDockerBuild(cfg openwrt.FirmwareConfig) error {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 OpenWrt 编译工作目录（建议 ≥40GB 空间）"})
	if err != nil || dir == "" {
		return fmt.Errorf("已取消选择目录")
	}
	pre := a.provider.RunPreflightCheck(dir)
	if pre.FreeDiskGB > 0 && pre.FreeDiskGB < 40 {
		return fmt.Errorf("前置预检拦截：所选目录剩余磁盘空间仅 %.1f GB，编译 OpenWrt 至少需要 40 GB", pre.FreeDiskGB)
	}
	go func() {
		a.pushOpenWrtLog("🐳 前置预检通过，正在启动本机 Docker 隔离编译...")
		if err := openwrt.RunDockerBuild(dir, cfg, a.pushOpenWrtLog); err != nil {
			a.pushOpenWrtLog("❌ Docker 编译中断: " + err.Error())
		} else {
			a.pushOpenWrtLog("🎉 编译完成！固件位于工作目录的 firmware_output 文件夹。")
		}
	}()
	return nil
}
func (a *App) PushOpenWrtToGitHub(req openwrt.GitHubPushRequest) (string, error) { return openwrt.PushAndTriggerWorkflow(req, a.pushOpenWrtLog) }
func (a *App) ProvisionOpenWrtSSH(req openwrt.SSHProvisionRequest) (string, error) { return openwrt.RunSSHProvision(req) }

func (a *App) GetSoftwareCatalog() ([]core.SoftwareItem, error) {
	if data, err := os.ReadFile(defaultCatalogFile); err == nil {
		var list []core.SoftwareItem
		if json.Unmarshal(data, &list) == nil && len(list) > 0 {
			return list, nil
		}
	}
	defaults := []core.SoftwareItem{
		{ID: "vscode", Name: "Visual Studio Code", Category: "Tool", TargetRole: "desktop", BinName: "code", Targets: core.PackageTargets{Winget: "Microsoft.VisualStudioCode", BrewCask: "visual-studio-code", Apt: "code", Dnf: "code", Pacman: "code"}},
		{ID: "gh_desktop", Name: "GitHub Desktop (图形化仓库同步)", Category: "Tool", TargetRole: "desktop", Targets: core.PackageTargets{Winget: "GitHub.GitHubDesktop", BrewCask: "github"}},
		{ID: "git", Name: "Git 版本控制", Category: "DevEnv", TargetRole: "all", BinName: "git", Targets: core.PackageTargets{Winget: "Git.Git", Brew: "git", Apt: "git", Dnf: "git", Pacman: "git"}, NeedMirror: true},
		{ID: "go", Name: "Go 语言环境", Category: "DevEnv", TargetRole: "all", BinName: "go", Targets: core.PackageTargets{Winget: "GoLang.Go", Brew: "go", Apt: "golang", Dnf: "golang", Pacman: "go"}, NeedMirror: true},
		{ID: "python", Name: "Python 3", Category: "DevEnv", TargetRole: "all", BinName: "python3", Targets: core.PackageTargets{Winget: "Python.Python.3.12", Brew: "python@3.12", Apt: "python3 python3-pip", Dnf: "python3 python3-pip", Pacman: "python python-pip"}, NeedMirror: true},
		{ID: "nodejs", Name: "Node.js LTS", Category: "DevEnv", TargetRole: "all", BinName: "node", Targets: core.PackageTargets{Winget: "OpenJS.NodeJS.LTS", Brew: "node", Apt: "nodejs npm", Dnf: "nodejs npm", Pacman: "nodejs npm"}, NeedMirror: true},
		{ID: "docker", Name: "Docker 容器引擎", Category: "Tool", TargetRole: "server", BinName: "docker", Targets: core.PackageTargets{Winget: "Docker.DockerDesktop", BrewCask: "docker", Apt: "docker.io docker-compose-v2", Dnf: "docker docker-compose", Pacman: "docker docker-compose"}, NeedMirror: true},
		{ID: "tailscale", Name: "Tailscale 点对点组网", Category: "Tool", TargetRole: "server", BinName: "tailscale", Targets: core.PackageTargets{Winget: "Tailscale.Tailscale", BrewCask: "tailscale", Apt: "tailscale", Dnf: "tailscale", Pacman: "tailscale"}},
		{ID: "sing-box", Name: "Sing-box 代理核心", Category: "Tool", TargetRole: "server", BinName: "sing-box", Targets: core.PackageTargets{Winget: "SagerNet.Sing-Box", Brew: "sing-box", Apt: "sing-box", Dnf: "sing-box", Pacman: "sing-box"}},
	}
	_ = a.SaveSoftwareCatalog(defaults)
	return defaults, nil
}
func (a *App) SaveSoftwareCatalog(list []core.SoftwareItem) error {
	data, _ := json.MarshalIndent(list, "", "  ")
	return os.WriteFile(defaultCatalogFile, data, 0644)
}
func (a *App) SearchPackageByWinget(kw string) (string, error) { return a.provider.SearchSoftware(kw) }

func (a *App) ExportConfigDialog(p config.SetupProfile) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "导出脱敏配置快照", DefaultFilename: "my-sysinit-profile.json"})
	if err != nil || path == "" {
		return "", err
	}
	p.NetExtra.TailscaleKey = ""
	return path, config.ExportProfile(path, &p)
}
func (a *App) ImportConfigDialog() (*config.SetupProfile, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择装机配置快照"})
	if err != nil || path == "" {
		return nil, err
	}
	return config.ImportProfile(path)
}

func (a *App) RunAllTasks(profile config.SetupProfile, hw *core.HardwareInfo) error {
	if !a.running.CompareAndSwap(false, true) {
		return fmt.Errorf("已有初始化任务正在执行中，请等待完成")
	}
	go func() {
		defer a.running.Store(false)
		a.ExecutePipelineSync(profile, hw)
	}()
	return nil
}

func (a *App) ExecutePipelineSync(profile config.SetupProfile, hw *core.HardwareInfo) {
	a.pushLog(5, "🔍 正在执行前置环境预检...")
	pre := a.provider.RunPreflightCheck("")
	for _, w := range pre.Warnings {
		a.pushLog(5, "⚠️ 预检提醒: "+w)
	}
	if profile.CreateBackup {
		a.pushLog(15, "正在创建系统还原点/快照...")
		_ = a.provider.CreateRestorePoint("SysInitPro 自动备份")
	}
	if profile.InstallDrivers && hw != nil {
		a.pushLog(28, "正在安装官方硬件驱动/微码...")
		_ = a.provider.InstallOfficialDrivers(hw)
	}
	if profile.AutoUpdateOS {
		a.pushLog(40, "🛡️ 正在执行系统补丁分级更新...")
		reboot, _ := a.provider.InstallSystemPatches(profile.SecurityOnly, func(s string) { a.pushLog(-1, "  | "+s) })
		if reboot {
			a.pushLog(45, "⚠️ 系统补丁需要重启生效，流水线结束后请手动重启。")
		}
	}
	if len(profile.OptimizeOptions) > 0 {
		a.pushLog(52, "正在应用系统差异化调优...")
		_ = a.provider.OptimizeSystem(profile.OptimizeOptions)
	}
	if profile.InstallRuntimes {
		a.pushLog(60, "正在补齐运行库与基础工具链...")
		_ = a.provider.InstallSystemRuntimes()
	}
	for i, item := range profile.SoftwareList {
		prog := 65 + int(float64(i+1)/float64(len(profile.SoftwareList))*23)
		a.pushLog(prog, fmt.Sprintf("=== [%d/%d] 处理: %s ===", i+1, len(profile.SoftwareList), item.Name))
		_ = a.provider.InstallSoftware(item, func(l string) { a.pushLog(-1, "  | "+l) })
		if item.Category == "DevEnv" {
			_ = a.provider.ConfigureDevEnvironment(item, profile.SelectedMirror)
		}
	}
	a.pushLog(92, "正在应用主机名 / Git / Swap / Tailscale 扩展配置...")
	_ = a.provider.ApplyExtraConfig(profile.DevExtra, profile.NetExtra, profile.TargetRole, func(l string) { a.pushLog(-1, "  | "+l) })
	a.pushLog(96, "🩺 正在运行装后环境体检...")
	for _, line := range a.provider.VerifyEnvironment(profile.SoftwareList) {
		a.pushLog(-1, "  "+line)
	}
	a.pushLog(100, "✅ 所有初始化任务与装后验收已全部完成！")
}