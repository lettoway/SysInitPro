package translator

import (
	"SysInitPro/config"
	"fmt"
	"strings"
)

func TranslateToPowerShell(p config.SetupProfile) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# SysInitPro v9.0 Windows 初始化剧本 (%s)\n$ErrorActionPreference = 'Continue'\n\n", p.ProfileName))
	if p.CreateBackup {
		sb.WriteString("Enable-ComputerRestore -Drive \"$env:SystemDrive\\\" -ErrorAction SilentlyContinue\nCheckpoint-Computer -Description 'SysInitPro 自动备份' -RestorePointType 'MODIFY_SETTINGS' -ErrorAction SilentlyContinue\n\n")
	}
	if h := strings.TrimSpace(p.NetExtra.CustomHostname); h != "" {
		sb.WriteString(fmt.Sprintf("Rename-Computer -NewName '%s' -Force -ErrorAction SilentlyContinue\n", h))
	}
	if p.NetExtra.DisableHibernat {
		sb.WriteString("powercfg -h off\n")
	}
	for _, opt := range p.OptimizeOptions {
		if opt == "desk_perf" || opt == "high_perf" {
			sb.WriteString("powercfg /setactive 8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c\n")
		} else if opt == "desk_telemetry" || opt == "disable_telemetry" {
			sb.WriteString("Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced' -Name 'HideFileExt' -Value 0 -Type DWord -Force\n")
		}
	}
	if p.InstallRuntimes {
		sb.WriteString("\nwinget install --id Microsoft.VCRedist.2015+.x64 --silent --accept-package-agreements --accept-source-agreements\n")
		sb.WriteString("winget install --id Microsoft.DotNet.DesktopRuntime.8 --silent --accept-package-agreements --accept-source-agreements\n")
	}
	for _, sw := range p.SoftwareList {
		if pkgID := strings.TrimSpace(sw.Targets.Winget); pkgID != "" {
			sb.WriteString(fmt.Sprintf("winget install --id %s --silent --accept-package-agreements --accept-source-agreements\n", pkgID))
		}
	}
	if p.SelectedMirror != "official" && p.SelectedMirror != "" {
		sb.WriteString("\nsetx GOPROXY \"https://goproxy.cn,direct\" /M\n")
		sb.WriteString(fmt.Sprintf("if (Get-Command pip -ErrorAction SilentlyContinue) { pip config set global.index-url '%s/pypi/web/simple' }\n", strings.TrimRight(p.SelectedMirror, "/")))
		sb.WriteString("if (Get-Command npm -ErrorAction SilentlyContinue) { npm config set registry https://registry.npmmirror.com }\n")
	}
	if p.DevExtra.GitUserName != "" {
		sb.WriteString(fmt.Sprintf("git config --global user.name \"%s\"\n", strings.TrimSpace(p.DevExtra.GitUserName)))
	}
	if p.DevExtra.GitUserEmail != "" {
		sb.WriteString(fmt.Sprintf("git config --global user.email \"%s\"\n", strings.TrimSpace(p.DevExtra.GitUserEmail)))
	}
	if p.NetExtra.TailscaleKey != "" {
		sb.WriteString(fmt.Sprintf("tailscale up --authkey=%s\n", strings.TrimSpace(p.NetExtra.TailscaleKey)))
	}
	sb.WriteString("\nWrite-Host '✅ 初始化完成！' -ForegroundColor Green\n")
	return sb.String()
}
