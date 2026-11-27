package pve

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type ProbeResult struct {
	OK      bool   `json:"ok"`
	Latency string `json:"latency"`
	Summary string `json:"summary"`
	Error   string `json:"error,omitempty"`
}

func ProbeHost(host, user, password string) ProbeResult {
	host = strings.TrimSpace(host)
	user = strings.TrimSpace(user)
	if host == "" {
		return ProbeResult{OK: false, Error: "主机地址不能为空"}
	}
	if user == "" {
		user = "root"
	}
	if !strings.Contains(host, ":") {
		host += ":22"
	}

	start := time.Now()
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return ProbeResult{OK: false, Error: "TCP 连接失败（10s 超时）: " + err.Error()}
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, host, cfg)
	if err != nil {
		_ = conn.Close()
		return ProbeResult{OK: false, Error: "SSH 认证失败: " + err.Error()}
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return ProbeResult{OK: false, Error: "创建会话失败: " + err.Error()}
	}
	defer sess.Close()

	out, _ := sess.CombinedOutput(`echo "Host: $(hostname) | Kernel: $(uname -r) | PVE: $(pveversion 2>/dev/null | head -n1 || echo N/A) | Mem: $(free -h | awk '/Mem:/{print $3"/"$2}')"`)
	return ProbeResult{
		OK:      true,
		Latency: fmt.Sprintf("%dms", time.Since(start).Milliseconds()),
		Summary: strings.TrimSpace(string(out)),
	}
}
