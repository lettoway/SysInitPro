package openwrt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ensureWindowsDockerPrereqs(logFn func(string)) error {
	requiredFeatures := []string{
		"Microsoft-Windows-Subsystem-Linux",
		"VirtualMachinePlatform",
	}
	needReboot := false

	for _, feat := range requiredFeatures {
		out, _ := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf(`(Get-WindowsOptionalFeature -Online -FeatureName '%s' -ErrorAction SilentlyContinue).State`, feat)).Output()
		if strings.TrimSpace(string(out)) == "Enabled" {
			continue
		}

		adminOut, _ := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
			`([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`).Output()
		if !strings.EqualFold(strings.TrimSpace(string(adminOut)), "True") {
			return fmt.Errorf("Windows 功能 [%s] 未开启，请以管理员身份重新运行 SysInitPro 以自动开启", feat)
		}

		if logFn != nil {
			logFn(fmt.Sprintf("⚠️ 检测到 Windows 功能 [%s] 未开启，正在自动启用...", feat))
		}
		err := exec.Command("dism.exe", "/online", "/enable-feature", "/featurename:"+feat, "/all", "/norestart").Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3010 {
				needReboot = true
			} else {
				return fmt.Errorf("自动启用 Windows 功能 [%s] 失败: %v", feat, err)
			}
		} else {
			needReboot = true
		}
		if logFn != nil {
			logFn(fmt.Sprintf("✅ 已启用 Windows 功能 [%s]", feat))
		}
	}

	if err := exec.Command("wsl", "--status").Run(); err != nil {
		if logFn != nil {
			logFn("⚠️ 未检测到就绪的 WSL 2 环境，正在自动下载并安装 WSL 2 内核...")
		}
		if installErr := exec.Command("wsl", "--install", "--no-distribution").Run(); installErr != nil {
			return fmt.Errorf("自动安装 WSL 2 失败（请确认以管理员身份运行）: %v", installErr)
		}
		needReboot = true
	}

	if needReboot {
		return fmt.Errorf("已自动启用 Windows 虚拟化/WSL 2 组件，请重启电脑后再启动 Docker 编译")
	}
	return nil
}

func RunDockerBuild(workDir string, cfg FirmwareConfig, logFn func(string)) error {
	if runtime.GOOS == "windows" {
		if err := ensureWindowsDockerPrereqs(logFn); err != nil {
			return err
		}
	}

	if _, err := exec.LookPath("docker"); err != nil {
		if runtime.GOOS == "windows" {
			if logFn != nil {
				logFn("⚠️ 未检测到 Docker，正在尝试通过 winget 自动静默安装 Docker Desktop...")
			}
			_ = exec.Command("winget", "install", "--id", "Docker.DockerDesktop", "--silent", "--accept-package-agreements", "--accept-source-agreements").Run()
			return fmt.Errorf("已触发 Docker Desktop 安装，请启动 Docker Desktop 后再试")
		}
		return fmt.Errorf("未检测到 Docker，请先安装并启动 Docker")
	}

	bundle := BuildFirmwareBundle(cfg)
	outDir := filepath.Join(workDir, "firmware_output")
	_ = os.MkdirAll(outDir, 0755)
	p1, p2, dc := filepath.Join(workDir, "diy-part1.sh"), filepath.Join(workDir, "diy-part2.sh"), filepath.Join(workDir, "diff.config")
	_ = os.WriteFile(p1, []byte(bundle.DiyPart1), 0755)
	_ = os.WriteFile(p2, []byte(bundle.DiyPart2), 0755)
	_ = os.WriteFile(dc, []byte(bundle.DotConfig), 0644)

	repoURL, repoBranch := resolveRepoInfo(cfg.Branch)
	innerCmd := fmt.Sprintf(`apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y build-essential clang flex bison g++ gawk gcc-multilib g++-multilib gettext git libncurses-dev libssl-dev python3-distutils rsync unzip zlib1g-dev file wget && useradd -m builder && su - builder -c 'git clone --depth 1 -b %s %s /home/builder/openwrt && cd /home/builder/openwrt && cp /work/diy-part1.sh . && chmod +x diy-part1.sh && ./diy-part1.sh && cp /work/diy-part2.sh /work/diff.config . && chmod +x diy-part2.sh && ./diy-part2.sh && cp diff.config .config && make defconfig && make download -j$(nproc) && (make -j$(nproc) || make -j1 V=s) && find bin/targets/ -type f \( -name "*.img.gz" -o -name "*.bin" \) -exec cp {} /work/output/ \;'`, repoBranch, repoURL)

	cmd := exec.Command("docker", "run", "--rm", "-v", p1+":/work/diy-part1.sh", "-v", p2+":/work/diy-part2.sh", "-v", dc+":/work/diff.config", "-v", outDir+":/work/output", "ubuntu:22.04", "bash", "-c", innerCmd)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 Docker 失败: %v", err)
	}
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" && logFn != nil {
			logFn(line)
		}
	}
	return cmd.Wait()
}