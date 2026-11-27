package openwrt

import (
	"SysInitPro/internal/engine/runner"
	"SysInitPro/internal/models"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ensureWindowsPrereqs(ctx context.Context, logFn func(string)) error {
	features := []string{"Microsoft-Windows-Subsystem-Linux", "VirtualMachinePlatform"}
	needReboot := false

	for _, feat := range features {
		out, _ := runner.ExecPS(ctx, fmt.Sprintf(`(Get-WindowsOptionalFeature -Online -FeatureName '%s' -ErrorAction SilentlyContinue).State`, feat))
		if strings.TrimSpace(out) == "Enabled" {
			continue
		}
		adminOut, _ := runner.ExecPS(ctx, `([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`)
		if !strings.EqualFold(strings.TrimSpace(adminOut), "True") {
			return fmt.Errorf("Windows 功能 [%s] 未开启，请以管理员身份运行 SysInitPro", feat)
		}
		if logFn != nil {
			logFn(fmt.Sprintf("⚠️ 正在自动启用 Windows 功能 [%s]...", feat))
		}
		if _, err := runner.Exec(ctx, "dism.exe", "/online", "/enable-feature", "/featurename:"+feat, "/all", "/norestart"); err != nil {
			if !strings.Contains(err.Error(), "3010") {
				return fmt.Errorf("启用 [%s] 失败: %v", feat, err)
			}
		}
		needReboot = true
	}

	if _, err := runner.Exec(ctx, "wsl", "--status"); err != nil {
		if logFn != nil {
			logFn("⚠️ 正在安装 WSL 2 内核...")
		}
		if _, iErr := runner.Exec(ctx, "wsl", "--install", "--no-distribution"); iErr != nil {
			return fmt.Errorf("自动安装 WSL 2 失败: %v", iErr)
		}
		needReboot = true
	}

	if needReboot {
		return fmt.Errorf("已启用 Windows 虚拟化/WSL 2 组件，请重启电脑后再启动 Docker 编译")
	}
	return nil
}

func RunDockerBuild(ctx context.Context, workDir string, cfg models.FirmwareConfig, logFn func(string)) error {
	if runtime.GOOS == "windows" {
		if err := ensureWindowsPrereqs(ctx, logFn); err != nil {
			return err
		}
	}

	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("未检测到 Docker，请先安装并启动 Docker Desktop")
	}

	bundle := BuildBundle(cfg)
	outDir := filepath.Join(workDir, "firmware_output")
	_ = os.MkdirAll(outDir, 0755)
	p1, p2, dc := filepath.Join(workDir, "diy-part1.sh"), filepath.Join(workDir, "diy-part2.sh"), filepath.Join(workDir, "diff.config")
	_ = os.WriteFile(p1, []byte(bundle.DiyPart1), 0755)
	_ = os.WriteFile(p2, []byte(bundle.DiyPart2), 0755)
	_ = os.WriteFile(dc, []byte(bundle.DotConfig), 0644)

	repoURL, repoBranch := ResolveRepoInfo(cfg.Branch, cfg.GitMirror)
	innerCmd := fmt.Sprintf(`set -e; apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y build-essential clang flex bison g++ gawk gcc-multilib g++-multilib gettext git libncurses-dev libssl-dev python3-distutils rsync unzip zlib1g-dev file wget && useradd -m builder && su - builder -c 'git clone --depth 1 -b %s %s /home/builder/openwrt && cd /home/builder/openwrt && cp /work/diy-part1.sh . && chmod +x diy-part1.sh && ./diy-part1.sh && cp /work/diy-part2.sh /work/diff.config . && chmod +x diy-part2.sh && ./diy-part2.sh && cp diff.config .config && make defconfig 2>&1 | tee defconfig.log; if grep -Eq "conflicts with|Recursive dependency" defconfig.log; then echo "[CONFLICT] 检测到致命依赖冲突，已中止编译"; grep -E "conflicts with|Recursive dependency" defconfig.log; exit 1; fi && make download -j$(nproc) && (make -j$(nproc) || make -j1 V=s) && find bin/targets/ -type f \( -name "*.img.gz" -o -name "*.bin" \) -exec cp {} /work/output/ \;'`, repoBranch, repoURL)

	if logFn != nil {
		logFn(fmt.Sprintf("🚀 启动 Docker 编译 [%s / %s]...", cfg.Target, cfg.Branch))
	}
	return runner.Stream(ctx, logFn, "docker", "run", "--rm",
		"-v", p1+":/work/diy-part1.sh",
		"-v", p2+":/work/diy-part2.sh",
		"-v", dc+":/work/diff.config",
		"-v", outDir+":/work/output",
		"ubuntu:22.04", "bash", "-c", innerCmd)
}
