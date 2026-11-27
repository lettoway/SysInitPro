//go:build linux

package platform

import (
	"SysInitPro/core"
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type LinuxProvider struct{ pkgManager string }

func NewSystemProvider() core.SystemProvider {
	pm := "apt"
	if _, err := exec.LookPath("dnf"); err == nil {
		pm = "dnf"
	} else if _, err := exec.LookPath("pacman"); err == nil {
		pm = "pacman"
	}
	return &LinuxProvider{pkgManager: pm}
}

func runShell(command string) (string, error) {
	out, err := exec.Command("bash", "-c", command).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func streamLinuxShell(command string, logFn func(string)) error {
	cmd := exec.Command("bash", "-c", command)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" && logFn != nil {
			logFn(line)
		}
	}
	return cmd.Wait()
}

func (l *LinuxProvider) RunPreflightCheck(workDir string) core.PreflightResult {
	res := core.PreflightResult{IsAdmin: os.Geteuid() == 0}
	if !res.IsAdmin {
		res.Warnings = append(res.Warnings, "当前非 root 权限，系统级安装可能需要 sudo")
	}
	if workDir == "" {
		workDir = "."
	}
	if out, err := runShell(fmt.Sprintf(`df -BG --output=avail '%s' | tail -n 1 | tr -dc '0-9'`, workDir)); err == nil {
		if f, err := strconv.ParseFloat(out, 64); err == nil {
			res.FreeDiskGB = f
			if f < 40 {
				res.Warnings = append(res.Warnings, fmt.Sprintf("当前目录剩余磁盘仅 %.0f GB（编译 OpenWrt 建议 ≥40 GB）", f))
			}
		}
	}
	return res
}

func (l *LinuxProvider) CreateRestorePoint(desc string) error {
	if _, err := exec.LookPath("timeshift"); err == nil {
		_, err := runShell(fmt.Sprintf(`timeshift --create --comments "%s"`, desc))
		return err
	}
	return fmt.Errorf("未检测到 timeshift，已跳过快照")
}

func (l *LinuxProvider) ScanHardware() (*core.HardwareInfo, error) {
	osVer, _ := runShell(`grep PRETTY_NAME /etc/os-release | cut -d'"' -f2`)
	cpu, _ := runShell(`lscpu | grep "Model name" | sed 's/Model name:\s*//'`)
	gpu, _ := runShell(`lspci | grep -iE 'vga|3d|display' | cut -d':' -f3`)
	board, _ := runShell(`cat /sys/devices/virtual/dmi/id/board_name 2>/dev/null || echo "Generic Linux"`)
	net, _ := runShell(`lspci | grep -iE 'network|ethernet' | cut -d':' -f3 | head -n 1`)
	role := "server"
	if out, err := runShell(`systemctl is-active display-manager 2>/dev/null`); err == nil && strings.Contains(out, "active") {
		role = "desktop"
	}
	return &core.HardwareInfo{
		OSPlatform: "linux", OSVersion: osVer, OSRole: role, ActivationStatus: "开源系统",
		CPU: cpu, GPU: gpu, Motherboard: board, NetworkCard: net,
	}, nil
}

func (l *LinuxProvider) InstallOfficialDrivers(hw *core.HardwareInfo) error {
	if hw != nil && hw.OSRole == "server" {
		if l.pkgManager == "apt" {
			_, _ = runShell(`DEBIAN_FRONTEND=noninteractive apt-get install -y intel-microcode amd64-microcode 2>/dev/null || true`)
		}
		return nil
	}
	if _, err := exec.LookPath("ubuntu-drivers"); err == nil {
		_, err := runShell(`ubuntu-drivers autoinstall`)
		return err
	}
	return nil
}

func (l *LinuxProvider) ScanSystemPatches() ([]core.PatchItem, error) {
	var items []core.PatchItem
	if l.pkgManager == "apt" {
		out, _ := runShell(`apt-get update >/dev/null 2>&1; apt list --upgradable 2>/dev/null | tail -n +2 | head -n 30`)
		for _, line := range strings.Split(out, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			pkg := strings.Split(line, "/")[0]
			cat := "Quality"
			if strings.Contains(strings.ToLower(line), "security") {
				cat = "Security"
			}
			items = append(items, core.PatchItem{ID: pkg, Title: line, Category: cat})
		}
	}
	return items, nil
}

func (l *LinuxProvider) InstallSystemPatches(securityOnly bool, logFn func(string)) (bool, error) {
	var err error
	switch l.pkgManager {
	case "apt":
		if securityOnly {
			err = streamLinuxShell(`DEBIAN_FRONTEND=noninteractive apt-get update && apt-get install -y unattended-upgrades && unattended-upgrade -d`, logFn)
		} else {
			err = streamLinuxShell(`DEBIAN_FRONTEND=noninteractive apt-get update && apt-get upgrade -y`, logFn)
		}
	case "dnf":
		if securityOnly {
			err = streamLinuxShell(`dnf upgrade --security -y`, logFn)
		} else {
			err = streamLinuxShell(`dnf upgrade -y`, logFn)
		}
	case "pacman":
		err = streamLinuxShell(`pacman -Syu --noconfirm`, logFn)
	}
	_, statErr := os.Stat("/var/run/reboot-required")
	return statErr == nil, err
}

func (l *LinuxProvider) OptimizeSystem(options []string) error {
	for _, opt := range options {
		switch opt {
		case "srv_bbr":
			_, _ = runShell(`grep -q bbr /etc/sysctl.conf || printf "net.core.default_qdisc=fq\nnet.ipv4.tcp_congestion_control=bbr\nnet.ipv4.ip_forward=1\n" >> /etc/sysctl.conf && sysctl -p`)
		case "srv_limits":
			_, _ = runShell(`sysctl -w vm.swappiness=10 && grep -q "65535" /etc/security/limits.conf || printf "* soft nofile 65535\n* hard nofile 65535\n" >> /etc/security/limits.conf`)
		case "srv_ssh_harden":
			if l.pkgManager == "apt" {
				_, _ = runShell(`DEBIAN_FRONTEND=noninteractive apt-get install -y fail2ban && systemctl enable --now fail2ban`)
			}
		case "desk_multimedia":
			if l.pkgManager == "apt" {
				_, _ = runShell(`DEBIAN_FRONTEND=noninteractive apt-get install -y fonts-noto-cjk ffmpeg`)
			}
		}
	}
	return nil
}

func (l *LinuxProvider) InstallSystemRuntimes() error {
	if l.pkgManager == "apt" {
		_, err := runShell(`DEBIAN_FRONTEND=noninteractive apt-get install -y build-essential curl wget git ca-certificates`)
		return err
	}
	return nil
}

func (l *LinuxProvider) SearchSoftware(keyword string) (string, error) {
	if l.pkgManager == "dnf" {
		return runShell(fmt.Sprintf(`dnf search "%s" | head -n 12`, keyword))
	}
	if l.pkgManager == "pacman" {
		return runShell(fmt.Sprintf(`pacman -Ss "%s" | head -n 12`, keyword))
	}
	return runShell(fmt.Sprintf(`apt-cache search "%s" | head -n 12`, keyword))
}

func (l *LinuxProvider) InstallSoftware(item core.SoftwareItem, logFn func(string)) error {
	if item.BinName != "" {
		if _, err := exec.LookPath(item.BinName); err == nil {
			if logFn != nil {
				logFn(fmt.Sprintf("[已就绪] 跳过 %s", item.BinName))
			}
			return nil
		}
	}
	if item.ID == "sing-box" {
		return streamLinuxShell(`curl -fsSL https://sing-box.app/install.sh | sh && systemctl enable sing-box`, logFn)
	}
	if item.ID == "tailscale" {
		return streamLinuxShell(`curl -fsSL https://tailscale.com/install.sh | sh`, logFn)
	}
	if l.pkgManager == "apt" && item.Targets.Apt != "" {
		return streamLinuxShell(`DEBIAN_FRONTEND=noninteractive apt-get install -y `+item.Targets.Apt, logFn)
	}
	if l.pkgManager == "dnf" && item.Targets.Dnf != "" {
		return streamLinuxShell(`dnf install -y `+item.Targets.Dnf, logFn)
	}
	if l.pkgManager == "pacman" && item.Targets.Pacman != "" {
		return streamLinuxShell(`pacman -S --noconfirm `+item.Targets.Pacman, logFn)
	}
	return nil
}

func (l *LinuxProvider) ConfigureDevEnvironment(item core.SoftwareItem, mirrorURL string) error {
	if mirrorURL == "official" || mirrorURL == "" {
		return nil
	}
	switch item.ID {
	case "go":
		_, _ = runShell(`go env -w GOPROXY=https://goproxy.cn,direct`)
	case "python":
		_, _ = runShell(fmt.Sprintf(`pip3 config set global.index-url "%s/pypi/web/simple"`, strings.TrimRight(mirrorURL, "/")))
	case "nodejs":
		_, _ = runShell(`npm config set registry https://registry.npmmirror.com`)
	}
	return nil
}

func (l *LinuxProvider) ApplyExtraConfig(dev core.DevEnvExtra, net core.NetworkExtra, role string, logFn func(string)) error {
	if h := strings.TrimSpace(net.CustomHostname); h != "" {
		_, _ = runShell(fmt.Sprintf(`hostnamectl set-hostname '%s' 2>/dev/null || true`, h))
	}
	if role == "server" && net.SwapSizeGB > 0 {
		_, _ = runShell(fmt.Sprintf(`if [ ! -f /swapfile ]; then fallocate -l %dG /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile && echo '/swapfile none swap sw 0 0' >> /etc/fstab; fi`, net.SwapSizeGB))
	}
	if dev.GitUserName != "" {
		_, _ = runShell(fmt.Sprintf(`git config --global user.name '%s'`, strings.TrimSpace(dev.GitUserName)))
	}
	if dev.GitUserEmail != "" {
		_, _ = runShell(fmt.Sprintf(`git config --global user.email '%s'`, strings.TrimSpace(dev.GitUserEmail)))
	}
	return nil
}

func (l *LinuxProvider) ExportInstalledSoftware() ([]core.SoftwareItem, error) {
	out, err := runShell(`apt-mark showmanual 2>/dev/null | head -n 25`)
	if err != nil {
		return nil, err
	}
	var items []core.SoftwareItem
	for _, pkg := range strings.Split(out, "\n") {
		if p := strings.TrimSpace(pkg); p != "" {
			items = append(items, core.SoftwareItem{ID: "local_" + p, Name: p + " (本机提取)", Category: "Tool", TargetRole: "all", BinName: p, Targets: core.PackageTargets{Apt: p}})
		}
	}
	return items, nil
}

func (l *LinuxProvider) VerifyEnvironment(items []core.SoftwareItem) []string {
	res := core.CommonVerifyEnvironment(items)
	if bbr, err := runShell(`sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null`); err == nil && bbr != "" {
		res = append(res, fmt.Sprintf("🌐 TCP 拥塞控制算法: %s", bbr))
	}
	return res
}
