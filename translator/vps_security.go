package translator

import (
	"SysInitPro/sshx"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var validLinuxUserRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

type VPSSecurityConfig struct {
	ServerAlias  string `json:"server_alias"`
	ServerIP     string `json:"server_ip"`
	InitUser     string `json:"init_user"`
	InitPassword string `json:"init_password"`
	OldSSHPort   string `json:"old_ssh_port"`
	NewSSHPort   string `json:"new_ssh_port"`
	NewUser      string `json:"new_user"`
	PublicKey    string `json:"public_key"`
	DisableRoot  bool   `json:"disable_root"`
	DisablePass  bool   `json:"disable_pass"`
	EnableUFW    bool   `json:"enable_ufw"`
	ExtraPorts   string `json:"extra_ports"`
}

func validatePort(portStr, fallback string) (string, error) {
	p := strings.TrimSpace(portStr)
	if p == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("无效端口号: %s", p)
	}
	return p, nil
}

func GenerateVPSSecurityScript(cfg VPSSecurityConfig) string {
	newUser := strings.TrimSpace(cfg.NewUser)
	if newUser == "" || !validLinuxUserRe.MatchString(newUser) {
		newUser = "deer"
	}
	newPort, err := validatePort(cfg.NewSSHPort, "26547")
	if err != nil {
		newPort = "26547"
	}
	var sb strings.Builder
	sb.WriteString("#!/usr/bin/env bash\nset -e\n")
	sb.WriteString("command -v apt-get >/dev/null && { DEBIAN_FRONTEND=noninteractive apt-get update && apt-get install -y sudo ufw; } || true\n")
	sb.WriteString(fmt.Sprintf("id -u %s >/dev/null 2>&1 || useradd -m -s /bin/bash %s\n", newUser, newUser))
	sb.WriteString(fmt.Sprintf("mkdir -p /home/%s/.ssh && chmod 700 /home/%s/.ssh\n", newUser, newUser))
	if pub := strings.TrimSpace(cfg.PublicKey); pub != "" {
		safePub := sshx.EscapeSingleQuote(pub)
		sb.WriteString(fmt.Sprintf("grep -qF '%s' /home/%s/.ssh/authorized_keys 2>/dev/null || echo '%s' >> /home/%s/.ssh/authorized_keys\n", safePub, newUser, safePub, newUser))
	}
	sb.WriteString(fmt.Sprintf("chmod 600 /home/%s/.ssh/authorized_keys && chown -R %s:%s /home/%s/.ssh\n", newUser, newUser, newUser, newUser))
	sb.WriteString(fmt.Sprintf("echo '%s ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/%s-nopasswd && chmod 0440 /etc/sudoers.d/%s-nopasswd && visudo -c -f /etc/sudoers.d/%s-nopasswd\n", newUser, newUser, newUser, newUser))
	if cfg.EnableUFW {
		sb.WriteString(fmt.Sprintf("ufw --force reset && ufw default deny incoming && ufw default allow outgoing && ufw allow %s/tcp\n", newPort))
		for _, p := range strings.Fields(cfg.ExtraPorts) {
			sb.WriteString(fmt.Sprintf("ufw allow %s\n", p))
		}
		sb.WriteString("ufw --force enable\n")
	}
	sb.WriteString("grep -qE '^#?PubkeyAuthentication ' /etc/ssh/sshd_config && sed -i 's/^#\\?PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config || echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config\n")
	sb.WriteString(fmt.Sprintf("grep -qE '^#?Port ' /etc/ssh/sshd_config && sed -i 's/^#\\?Port .*/Port %s/' /etc/ssh/sshd_config || echo 'Port %s' >> /etc/ssh/sshd_config\n", newPort, newPort))
	if cfg.DisableRoot {
		sb.WriteString("grep -qE '^#?PermitRootLogin ' /etc/ssh/sshd_config && sed -i 's/^#\\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config || echo 'PermitRootLogin no' >> /etc/ssh/sshd_config\n")
	}
	if cfg.DisablePass {
		sb.WriteString("grep -qE '^#?PasswordAuthentication ' /etc/ssh/sshd_config && sed -i 's/^#\\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config || echo 'PasswordAuthentication no' >> /etc/ssh/sshd_config\n")
		sb.WriteString("sed -i 's/PasswordAuthentication yes/PasswordAuthentication no/g' /etc/ssh/sshd_config.d/*.conf 2>/dev/null || true\n")
	}
	sb.WriteString(fmt.Sprintf("sshd -t && sshd -T | grep -q '^port %s$' && (systemctl restart sshd || systemctl restart ssh)\n", newPort))
	sb.WriteString(fmt.Sprintf("echo '✅ 加固完成！请使用新端口连接：ssh %s@%s -p %s'\n", newUser, cfg.ServerIP, newPort))
	return sb.String()
}

func EnsureLocalSSHKeyAndConfig(cfg VPSSecurityConfig) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	sshDir := filepath.Join(home, ".ssh")
	_ = os.MkdirAll(sshDir, 0700)
	privKey, pubKeyPath := filepath.Join(sshDir, "id_ed25519"), filepath.Join(sshDir, "id_ed25519.pub")
	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		if out, runErr := exec.Command("ssh-keygen", "-t", "ed25519", "-f", privKey, "-N", "", "-C", "sysinitpro@local").CombinedOutput(); runErr != nil {
			return "", "", fmt.Errorf("生成密钥失败: %v (%s)", runErr, string(out))
		}
	}
	rawPub, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return "", "", err
	}
	var cfgPath string
	if alias, ip := strings.TrimSpace(cfg.ServerAlias), strings.TrimSpace(cfg.ServerIP); alias != "" && ip != "" {
		cfgPath = filepath.Join(sshDir, "config")
		existing, _ := os.ReadFile(cfgPath)
		if !strings.Contains(string(existing), fmt.Sprintf("Host %s\n", alias)) {
			newUser := strings.TrimSpace(cfg.NewUser)
			if newUser == "" {
				newUser = "deer"
			}
			newPort, _ := validatePort(cfg.NewSSHPort, "26547")
			block := fmt.Sprintf("\nHost %s\n  HostName %s\n  User %s\n  Port %s\n  IdentityFile %s\n", alias, ip, newUser, newPort, filepath.ToSlash(privKey))
			if f, err := os.OpenFile(cfgPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
				_, _ = f.WriteString(block)
				_ = f.Close()
			}
		}
	}
	return strings.TrimSpace(string(rawPub)), cfgPath, nil
}

func RunVPSSSHProvision(cfg VPSSecurityConfig) (string, error) {
	if strings.TrimSpace(cfg.ServerIP) == "" {
		return "", fmt.Errorf("服务器公网 IP 不能为空")
	}
	oldPort, err := validatePort(cfg.OldSSHPort, "22")
	if err != nil {
		return "", err
	}
	pubKey, cfgPath, err := EnsureLocalSSHKeyAndConfig(cfg)
	if err != nil {
		return "", err
	}
	cfg.PublicKey = pubKey
	out, err := sshx.RunScript(sshx.SSHTarget{Host: cfg.ServerIP, Port: oldPort, User: cfg.InitUser, Password: cfg.InitPassword}, GenerateVPSSecurityScript(cfg), true)
	if err != nil {
		return out, fmt.Errorf("远端加固执行失败: %v", err)
	}
	if cfgPath != "" {
		out += fmt.Sprintf("\n📁 已写入本机 SSH 配置: %s", cfgPath)
	}
	return out, nil
}