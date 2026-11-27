package sshx

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type LocalSSHHost struct {
	Alias        string `json:"alias"`
	HostName     string `json:"hostname"`
	User         string `json:"user"`
	Port         string `json:"port"`
	IdentityFile string `json:"identity_file"`
}

type SSHTarget struct {
	Host         string `json:"host"`
	Port         string `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	IdentityFile string `json:"identity_file"`
}

func EscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\"'\"'")
}

func ListLocalSSHHosts() []LocalSSHHost {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	f, err := os.Open(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var hosts []LocalSSHHost
	var cur *LocalSSHHost
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key, val := strings.ToLower(fields[0]), strings.Join(fields[1:], " ")
		if key == "host" {
			if cur != nil && cur.Alias != "" && cur.Alias != "*" {
				hosts = append(hosts, *cur)
			}
			cur = &LocalSSHHost{Alias: fields[1], User: "root", Port: "22"}
		} else if cur != nil {
			switch key {
			case "hostname":
				cur.HostName = val
			case "user":
				cur.User = val
			case "port":
				cur.Port = val
			case "identityfile":
				if strings.HasPrefix(val, "~/") {
					val = filepath.Join(home, val[2:])
				}
				cur.IdentityFile = val
			}
		}
	}
	if cur != nil && cur.Alias != "" && cur.Alias != "*" {
		hosts = append(hosts, *cur)
	}
	return hosts
}

func Dial(t SSHTarget) (*ssh.Client, error) {
	if strings.TrimSpace(t.Host) == "" {
		return nil, fmt.Errorf("目标主机 IP 不能为空")
	}
	user := strings.TrimSpace(t.User)
	if user == "" {
		user = "root"
	}
	port := strings.TrimSpace(t.Port)
	if port == "" {
		port = "22"
	}

	var authMethods []ssh.AuthMethod
	keyPath := strings.TrimSpace(t.IdentityFile)
	if keyPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defKey := filepath.Join(home, ".ssh", "id_ed25519")
			if _, statErr := os.Stat(defKey); statErr == nil {
				keyPath = defKey
			}
		}
	}
	if keyPath != "" {
		if keyBytes, err := os.ReadFile(keyPath); err == nil {
			if signer, parseErr := ssh.ParsePrivateKey(keyBytes); parseErr == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}
	if t.Password != "" {
		authMethods = append(authMethods, ssh.Password(t.Password))
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未提供密码且未找到可用的本地 SSH 私钥")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", fmt.Sprintf("%s:%s", strings.TrimSpace(t.Host), port), cfg)
}

func RunCommand(t SSHTarget, cmdStr string) (string, error) {
	client, err := Dial(t)
	if err != nil {
		return "", err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Run(cmdStr); err != nil {
		return stdout.String() + "\n" + stderr.String(), err
	}
	return stdout.String(), nil
}

// RunScript 通过 Stdin 管道安全执行多行 Bash 脚本，自动兼容非 root 用户的 sudo -S 提权
func RunScript(t SSHTarget, script string, useSudo bool) (string, error) {
	client, err := Dial(t)
	if err != nil {
		return "", err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	session.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	execCmd := "bash -s"
	user := strings.TrimSpace(t.User)
	if useSudo && user != "" && user != "root" {
		if t.Password != "" {
			execCmd = fmt.Sprintf("printf '%%s\\n' '%s' | sudo -S bash -s", EscapeSingleQuote(t.Password))
		} else {
			execCmd = "sudo bash -s"
		}
	}

	if err := session.Run(execCmd); err != nil {
		return stdout.String() + "\n" + stderr.String(), err
	}
	return stdout.String(), nil
}

// UploadFile 通过 SSH 流式管道上传文件（零额外依赖，完美兼容 OpenWrt/BusyBox）
func UploadFile(t SSHTarget, localPath, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer f.Close()

	client, err := Dial(t)
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = f
	var stderr bytes.Buffer
	session.Stderr = &stderr
	if err := session.Run(fmt.Sprintf("cat > '%s'", EscapeSingleQuote(remotePath))); err != nil {
		return fmt.Errorf("上传文件失败: %v (%s)", err, stderr.String())
	}
	return nil
}
