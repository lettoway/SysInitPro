package main

import (
	"SysInitPro/core"
	"SysInitPro/internal/engine/store"
	"SysInitPro/network"
	"SysInitPro/platform"
	"SysInitPro/pve"
	"SysInitPro/sshx"
	"SysInitPro/translator"
	"context"
	"os"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) emitV10Log(line string) {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "v10:log", line)
	}
}

// --- 0. 后端加密凭据存储 ---

func (a *App) V10SaveSecret(key, value string) error {
	return store.SaveSecret(key, strings.TrimSpace(value))
}

func (a *App) V10LoadSecret(key string) string {
	return store.LoadSecret(key)
}

// --- 1. 系统初始化与硬件扫描 ---

func (a *App) V10ScanHardware() (*core.HardwareInfo, error) {
	return platform.NewSystemProvider().ScanHardware()
}

func (a *App) V10RunPreflight() core.PreflightResult {
	wd, _ := os.Getwd()
	return platform.NewSystemProvider().RunPreflightCheck(wd)
}

func (a *App) V10ScanPatches() ([]core.PatchItem, error) {
	return platform.NewSystemProvider().ScanSystemPatches()
}

func (a *App) V10InstallPatches(securityOnly bool) string {
	err := v10TaskMgr.Run("sys-patches", "安装系统更新补丁", func(ctx context.Context) error {
		_, e := platform.NewSystemProvider().InstallSystemPatches(securityOnly, a.emitV10Log)
		return e
	})
	if err != nil {
		return "[FAIL] " + err.Error()
	}
	return "[OK] 系统补丁安装任务已完成"
}

func (a *App) V10InstallRuntimes() string {
	err := v10TaskMgr.Run("sys-runtimes", "安装系统运行库", func(ctx context.Context) error {
		return platform.NewSystemProvider().InstallSystemRuntimes()
	})
	if err != nil {
		return "[FAIL] " + err.Error()
	}
	return "[OK] 系统运行库安装完成"
}

func (a *App) V10SearchSoftware(keyword string) (string, error) {
	return platform.NewSystemProvider().SearchSoftware(keyword)
}

func (a *App) V10BenchmarkMirrors() network.MirrorNode {
	return network.BenchmarkMirrors()
}

func (a *App) V10GetAllMirrors() []network.MirrorNode {
	return network.GetAllMirrors()
}

// --- 2. PVE 集群编排（统一接入 v10TaskMgr 实时日志流） ---

func (a *App) V10GetStructTemplates() map[string]interface{} {
	return map[string]interface{}{
		"pve": pve.PVEClusterConfig{},
		"vps": translator.VPSSecurityConfig{},
	}
}

func (a *App) runStreamedTask(id, name string, fn func() (string, error)) (string, error) {
	var out string
	a.emitV10Log(">>> [" + name + "] 任务启动...")
	err := v10TaskMgr.Run(id, name, func(ctx context.Context) error {
		res, e := fn()
		out = res
		if res != "" {
			for _, line := range strings.Split(res, "\n") {
				a.emitV10Log(line)
			}
		}
		return e
	})
	if err != nil {
		a.emitV10Log("[FAIL] [" + name + "] " + err.Error())
		return out, err
	}
	a.emitV10Log("[OK] [" + name + "] 执行完成")
	return out, nil
}

func (a *App) V10UpgradePVEHost(cfg pve.PVEClusterConfig) (string, error) {
	return a.runStreamedTask("pve-upgrade-host", "升级 PVE 宿主机", func() (string, error) {
		return pve.UpgradePVEHost(cfg)
	})
}

func (a *App) V10ConfigurePVEBootOrder(cfg pve.PVEClusterConfig) (string, error) {
	return a.runStreamedTask("pve-boot-order", "配置 VM 启动顺序", func() (string, error) {
		return pve.ConfigureBootOrder(cfg)
	})
}

func (a *App) V10SafeUpgradePVEVMs(cfg pve.PVEClusterConfig) (string, error) {
	return a.runStreamedTask("pve-upgrade-vms", "安全升级虚拟机", func() (string, error) {
		return pve.SafeUpgradeVMs(cfg)
	})
}

// --- 3. Sing-box 与 VPS 安全加固 ---

func (a *App) V10ParseSingboxURI(rawURI string) (*translator.SingboxFormConfig, error) {
	return translator.ParseSingboxURI(rawURI)
}

func (a *App) V10GenerateSingboxJSON(cfg translator.SingboxFormConfig) (string, error) {
	return translator.GenerateSingboxJSON(cfg)
}

func (a *App) V10GenerateVPSSecurity(cfg translator.VPSSecurityConfig) string {
	return translator.GenerateVPSSecurityScript(cfg)
}

func (a *App) V10RunVPSSSHProvision(cfg translator.VPSSecurityConfig) (string, error) {
	return a.runStreamedTask("vps-ssh-provision", "VPS SSH 安全加固", func() (string, error) {
		return translator.RunVPSSSHProvision(cfg)
	})
}

func (a *App) V10DeploySingboxToVPS(host string, port string, user string, password string, sbCfg translator.SingboxFormConfig) (string, error) {
	port = strings.TrimSpace(port)
	if port == "" {
		port = "22"
	}
	user = strings.TrimSpace(user)
	if user == "" {
		user = "root"
	}
	target := sshx.SSHTarget{
		Host:     strings.TrimSpace(host),
		Port:     port,
		User:     user,
		Password: password,
	}
	return a.runStreamedTask("singbox-deploy", "部署 Sing-box 至 VPS", func() (string, error) {
		return translator.DeploySingboxToVPS(target, sbCfg)
	})
}
