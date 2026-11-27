package translator

import (
	"SysInitPro/config"
	"fmt"
	"strings"
)

func TranslateToUnixShell(p config.SetupProfile) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("#!/usr/bin/env bash\n# SysInitPro v9.0 Unix/Linux 剧本 (%s)\nset -e\n\n", p.ProfileName))
	sb.WriteString("OS_TYPE=$(uname -s)\nPKG_MGR=''\n[ \"$OS_TYPE\" = 'Darwin' ] && PKG_MGR='brew' || { command -v apt-get >/dev/null && PKG_MGR='apt' || { command -v dnf >/dev/null && PKG_MGR='dnf' || PKG_MGR='pacman'; }; }\n\n")
	if h := strings.TrimSpace(p.NetExtra.CustomHostname); h != "" {
		sb.WriteString(fmt.Sprintf("hostnamectl set-hostname '%s' 2>/dev/null || true\n", h))
	}
	if p.TargetRole == "server" && p.NetExtra.SwapSizeGB > 0 {
		sb.WriteString(fmt.Sprintf("if [ \"$OS_TYPE\" = 'Linux' ] && [ ! -f /swapfile ]; then fallocate -l %dG /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile && echo '/swapfile none swap sw 0 0' >> /etc/fstab; fi\n", p.NetExtra.SwapSizeGB))
	}
	for _, opt := range p.OptimizeOptions {
		if opt == "srv_bbr" {
			sb.WriteString("if [ \"$OS_TYPE\" = 'Linux' ]; then grep -q bbr /etc/sysctl.conf || printf 'net.core.default_qdisc=fq\\nnet.ipv4.tcp_congestion_control=bbr\\nnet.ipv4.ip_forward=1\\n' >> /etc/sysctl.conf && sysctl -p; fi\n")
		}
	}
	sb.WriteString("[ \"$PKG_MGR\" = 'apt' ] && apt-get update || true\n")
	for _, sw := range p.SoftwareList {
		if sw.ID == "sing-box" {
			sb.WriteString("curl -fsSL https://sing-box.app/install.sh | sh && systemctl enable sing-box\n")
			continue
		}
		if sw.ID == "tailscale" {
			sb.WriteString("curl -fsSL https://tailscale.com/install.sh | sh\n")
			continue
		}
		if sw.Targets.Apt != "" {
			sb.WriteString(fmt.Sprintf("[ \"$PKG_MGR\" = 'apt' ] && DEBIAN_FRONTEND=noninteractive apt-get install -y %s || true\n", sw.Targets.Apt))
		}
		if sw.Targets.Dnf != "" {
			sb.WriteString(fmt.Sprintf("[ \"$PKG_MGR\" = 'dnf' ] && dnf install -y %s || true\n", sw.Targets.Dnf))
		}
		if sw.Targets.Pacman != "" {
			sb.WriteString(fmt.Sprintf("[ \"$PKG_MGR\" = 'pacman' ] && pacman -S --noconfirm %s || true\n", sw.Targets.Pacman))
		}
		if sw.Targets.Brew != "" {
			sb.WriteString(fmt.Sprintf("[ \"$PKG_MGR\" = 'brew' ] && brew install %s || true\n", sw.Targets.Brew))
		}
	}
	sb.WriteString("\necho '✅ 所有系统初始化任务已完成！'\n")
	return sb.String()
}
