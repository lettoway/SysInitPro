package pve

import (
	"SysInitPro/sshx"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type PVEClusterConfig struct {
	PVESSH      sshx.SSHTarget `json:"pve_ssh"`
	IKuaiVMID   string         `json:"ikuai_vmid"`
	OpenWrtVMID string         `json:"openwrt_vmid"`
	OpenWrtSSH  sshx.SSHTarget `json:"openwrt_ssh"`
	ZorinVMID   string         `json:"zorin_vmid"`
	ZorinSSH    sshx.SSHTarget `json:"zorin_ssh"`
	RepoSource  string         `json:"repo_source"` // "official" | "tsinghua" | "ustc"
}

func UpgradePVEHost(cfg PVEClusterConfig) (string, error) {
	baseURL := "http://download.proxmox.com/debian/pve"
	switch cfg.RepoSource {
	case "tsinghua":
		baseURL = "https://mirrors.tuna.tsinghua.edu.cn/proxmox/debian/pve"
	case "ustc":
		baseURL = "https://mirrors.ustc.edu.cn/proxmox/debian/pve"
	}
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -e
TS=$(date +%%Y%%m%%d-%%H%%M)
CODENAME=$(. /etc/os-release && echo "${VERSION_CODENAME:-bookworm}")
echo ">>> [1/4] 识别 Debian 代号: ${CODENAME}，备份配置至 /root/pve-backup-${TS}.tar.gz..."
tar -czf /root/pve-backup-${TS}.tar.gz /etc/pve /etc/network/interfaces 2>/dev/null || true
if command -v zfs >/dev/null 2>&1 && zfs list rpool/ROOT/pve-1 >/dev/null 2>&1; then
    zfs snapshot rpool/ROOT/pve-1@pre-upgrade-${TS}
    echo ">>> 已创建 ZFS 系统盘快照: pre-upgrade-${TS}"
fi
echo ">>> [2/4] 屏蔽企业源并配置无订阅源 (%s)..."
sed -i 's/^deb/#deb/g' /etc/apt/sources.list.d/pve-enterprise.list 2>/dev/null || true
sed -i 's/^deb/#deb/g' /etc/apt/sources.list.d/ceph.list 2>/dev/null || true
echo "deb %s ${CODENAME} pve-no-subscription" > /etc/apt/sources.list.d/pve-no-subscription.list
echo ">>> [3/4] 执行 PVE dist-upgrade..."
apt-get update && DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade -y
echo ">>> [4/4] 检测重启标记..."
if [ -f /var/run/reboot-required ]; then
    echo "⚠️ [需要重启] 本次更新包含新内核，请在低峰期手动重启或执行 shutdown -r 04:00"
else
    echo "✅ PVE 宿主机已更新完毕，无需重启，网络零中断！"
fi
`, baseURL, baseURL)
	return sshx.RunScript(cfg.PVESSH, script, true)
}

func ConfigureBootOrder(cfg PVEClusterConfig) (string, error) {
	var cmds []string
	if id := strings.TrimSpace(cfg.IKuaiVMID); id != "" {
		cmds = append(cmds, fmt.Sprintf("qm set %s --onboot 1 --startup order=1,up=20", id))
	}
	if id := strings.TrimSpace(cfg.OpenWrtVMID); id != "" {
		cmds = append(cmds, fmt.Sprintf("qm set %s --onboot 1 --startup order=2,up=15", id))
	}
	if id := strings.TrimSpace(cfg.ZorinVMID); id != "" {
		cmds = append(cmds, fmt.Sprintf("qm set %s --onboot 1 --startup order=3", id))
	}
	if len(cmds) == 0 {
		return "", fmt.Errorf("未配置任何 VMID")
	}
	return sshx.RunCommand(cfg.PVESSH, strings.Join(cmds, " && ")+" && echo '✅ 启动顺序已锁定：iKuai(1) -> OpenWrt(2) -> Zorin OS(3)'")
}

func SafeUpgradeVMs(cfg PVEClusterConfig) (string, error) {
	var logs strings.Builder
	snapName := "auto_" + time.Now().Format("20060102_1504")

	for _, id := range []string{cfg.IKuaiVMID, cfg.OpenWrtVMID, cfg.ZorinVMID} {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out, err := sshx.RunCommand(cfg.PVESSH, fmt.Sprintf("qm snapshot %s %s --description 'SysInitPro Auto Backup'", id, snapName))
		if err != nil {
			return logs.String(), fmt.Errorf("VM %s 创建快照失败: %v (%s)", id, err, out)
		}
		logs.WriteString(fmt.Sprintf("📸 VM %s 快照创建成功: %s\n", id, snapName))
	}

	if strings.TrimSpace(cfg.ZorinSSH.Host) != "" {
		logs.WriteString(">>> 正在升级 Zorin OS 系统软件包...\n")
		out, err := sshx.RunScript(cfg.ZorinSSH, "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y", true)
		if err != nil {
			logs.WriteString(fmt.Sprintf("❌ Zorin OS 升级异常，PVE 自动回滚快照 %s...\n", snapName))
			_, _ = sshx.RunCommand(cfg.PVESSH, fmt.Sprintf("qm rollback %s %s && qm start %s", cfg.ZorinVMID, snapName, cfg.ZorinVMID))
			return logs.String() + out, fmt.Errorf("Zorin OS 升级失败并已自动回滚: %v", err)
		}
		logs.WriteString("✅ Zorin OS 系统更新完成\n")
	}

	if owIP := strings.TrimSpace(cfg.OpenWrtSSH.Host); owIP != "" {
		if _, err := sshx.RunCommand(cfg.PVESSH, fmt.Sprintf("ping -c 3 -W 2 %s >/dev/null", owIP)); err != nil {
			logs.WriteString(fmt.Sprintf("⚠️ OpenWrt (%s) 失联，触发自动回滚...\n", owIP))
			_, _ = sshx.RunCommand(cfg.PVESSH, fmt.Sprintf("qm rollback %s %s && qm start %s", cfg.OpenWrtVMID, snapName, cfg.OpenWrtVMID))
			return logs.String(), fmt.Errorf("OpenWrt 失联，已回滚快照")
		}
		logs.WriteString(fmt.Sprintf("✅ OpenWrt (%s) 连通性检测正常\n", owIP))
	}

	logs.WriteString("🎉 软路由集群安全巡检与升级全部完成！\n")
	return logs.String(), nil
}

// UpgradeOpenWrtFirmware 闭环升级：PVE 打快照 -> 上传固件 -> sysupgrade -v -> 等待重启 -> 探针校验/自动回滚
func UpgradeOpenWrtFirmware(cfg PVEClusterConfig, localFirmwarePath string) (string, error) {
	var logs strings.Builder
	owID := strings.TrimSpace(cfg.OpenWrtVMID)
	owIP := strings.TrimSpace(cfg.OpenWrtSSH.Host)
	if owID == "" || owIP == "" {
		return "", fmt.Errorf("请先填写 OpenWrt 的 VMID 与 SSH IP 地址")
	}

	snapName := "fw_" + time.Now().Format("20060102_1504")
	logs.WriteString(fmt.Sprintf("📸 [1/4] 正在通过 PVE 为 OpenWrt (VM %s) 创建升级前快照 %s...\n", owID, snapName))
	if out, err := sshx.RunCommand(cfg.PVESSH, fmt.Sprintf("qm snapshot %s %s --description 'Pre-Sysupgrade'", owID, snapName)); err != nil {
		return logs.String(), fmt.Errorf("创建快照失败: %v (%s)", err, out)
	}

	remotePath := "/tmp/" + filepath.Base(localFirmwarePath)
	logs.WriteString(fmt.Sprintf("📤 [2/4] 正在上传固件至 OpenWrt (%s)...\n", remotePath))
	if err := sshx.UploadFile(cfg.OpenWrtSSH, localFirmwarePath, remotePath); err != nil {
		return logs.String(), err
	}

	logs.WriteString("🚀 [3/4] 正在触发 sysupgrade -v（保留现有网络与插件配置）并等待重启...\n")
	_, _ = sshx.RunCommand(cfg.OpenWrtSSH, fmt.Sprintf("sysupgrade -v '%s' >/dev/null 2>&1 &", sshx.EscapeSingleQuote(remotePath)))

	time.Sleep(60 * time.Second)
	logs.WriteString("🩺 [4/4] 正在从 PVE 宿主机检测 OpenWrt 重启后连通性...\n")
	alive := false
	for i := 0; i < 6; i++ {
		if _, err := sshx.RunCommand(cfg.PVESSH, fmt.Sprintf("ping -c 2 -W 2 %s >/dev/null", owIP)); err == nil {
			alive = true
			break
		}
		time.Sleep(10 * time.Second)
	}

	if !alive {
		logs.WriteString("❌ OpenWrt 升级后未恢复响应，PVE 正在自动回滚至升级前快照...\n")
		_, _ = sshx.RunCommand(cfg.PVESSH, fmt.Sprintf("qm rollback %s %s && qm start %s", owID, snapName, owID))
		return logs.String(), fmt.Errorf("OpenWrt 固件升级失联，已自动回滚快照 %s", snapName)
	}

	logs.WriteString("✅ OpenWrt 新固件已成功启动并通过连通性验证！\n")
	return logs.String(), nil
}

func DeploySubBoostToZorin(zorinSSH sshx.SSHTarget) (string, error) {
	if strings.TrimSpace(zorinSSH.Host) == "" {
		return "", fmt.Errorf("请先填写 Zorin OS 的 SSH 地址")
	}
	script := `#!/usr/bin/env bash
set -e
echo ">>> [1/3] 检查 Docker 环境..."
if ! command -v docker >/dev/null 2>&1; then
    echo ">>> 未检测到 Docker，正在自动安装..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
fi
echo ">>> [2/3] 生成 /opt/subboost/docker-compose.yml..."
mkdir -p /opt/subboost/subboost-data
cat << 'EOF' > /opt/subboost/docker-compose.yml
services:
  subboost:
    image: ghcr.io/subboost/subboost:latest
    container_name: subboost
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./subboost-data:/app/data
    environment:
      - TZ=Asia/Shanghai
EOF
echo ">>> [3/3] 启动 SubBoost 容器..."
cd /opt/subboost && docker compose pull && docker compose up -d
echo "✅ SubBoost 已启动！请浏览器访问: http://$(hostname -I | awk '{print $1}'):8080"
`
	return sshx.RunScript(zorinSSH, script, true)
}