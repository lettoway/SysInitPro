//go:build windows

package platform

import (
	"SysInitPro/core"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var winKeyRe = regexp.MustCompile(`^[A-Za-z0-9-]{25,29}$`)

type WindowsProvider struct{}

func NewSystemProvider() core.SystemProvider { return &WindowsProvider{} }

func runPS(script string) (string, error) {
	utf8Script := "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8; " + script
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", utf8Script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), fmt.Errorf("%v: %s", err, stderr.String())
	}
	return strings.TrimSpace(out.String()), nil
}

func (w *WindowsProvider) EnsureWinget(logFn func(string)) error {
	if _, err := exec.LookPath("winget"); err == nil {
		return nil
	}
	if logFn != nil {
		logFn("⚠️ 未检测到 winget，正在从微软官方源自动下载并静默安装...")
	}
	script := `
		$ProgressPreference = 'SilentlyContinue'
		$vc = Join-Path $env:TEMP "VCLibs.appx"
		$wg = Join-Path $env:TEMP "AppInstaller.msixbundle"
		Invoke-WebRequest -Uri "https://aka.ms/Microsoft.VCLibs.x64.14.00.Desktop.appx" -OutFile $vc
		Invoke-WebRequest -Uri "https://aka.ms/getwinget" -OutFile $wg
		Add-AppxPackage -Path $vc -ErrorAction SilentlyContinue
		Add-AppxPackage -Path $wg
		Remove-Item $vc, $wg -Force -ErrorAction SilentlyContinue
	`
	if _, err := runPS(script); err != nil {
		return fmt.Errorf("自动安装 winget 失败: %v", err)
	}
	// 刷新当前进程 PATH，确保立即可找到 winget.exe
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		appsDir := filepath.Join(localApp, "Microsoft", "WindowsApps")
		if curPath := os.Getenv("PATH"); !strings.Contains(strings.ToLower(curPath), strings.ToLower(appsDir)) {
			_ = os.Setenv("PATH", curPath+";"+appsDir)
		}
	}
	return nil
}

func (w *WindowsProvider) RunPreflightCheck(workDir string) core.PreflightResult {
	res := core.PreflightResult{}
	adminOut, _ := runPS(`([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`)
	res.IsAdmin = strings.EqualFold(adminOut, "True")
	if !res.IsAdmin {
		res.Warnings = append(res.Warnings, "当前未以管理员身份运行，部分系统调优与补丁安装可能受限")
	}

	drive := "C"
	if len(workDir) >= 2 && workDir[1] == ':' {
		drive = strings.ToUpper(workDir[:1])
	}
	freeStr, _ := runPS(fmt.Sprintf(`[math]::Round((Get-PSDrive -Name '%s').Free / 1GB, 1)`, drive))
	if f, err := strconv.ParseFloat(freeStr, 64); err == nil {
		res.FreeDiskGB = f
		if f < 40 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s 盘剩余空间仅 %.1f GB（Docker 编译 OpenWrt 建议至少 40 GB）", drive, f))
		}
	}
	return res
}

func (w *WindowsProvider) CreateRestorePoint(desc string) error {
	_, err := runPS(fmt.Sprintf(`Enable-ComputerRestore -Drive "$env:SystemDrive\" -ErrorAction SilentlyContinue; Checkpoint-Computer -Description "%s" -RestorePointType "MODIFY_SETTINGS"`, desc))
	return err
}

func (w *WindowsProvider) ScanHardware() (*core.HardwareInfo, error) {
	osVer, _ := runPS(`(Get-CimInstance Win32_OperatingSystem).Caption`)
	actStatus, _ := runPS(`$lic = Get-CimInstance SoftwareLicensingProduct -Filter "ApplicationID='55c92734-d682-4d71-983e-d6ec3f16059f' and PartialProductKey is not null" | Select-Object -First 1; if ($lic -and $lic.LicenseStatus -eq 1) { "已激活 (Licensed)" } else { "未激活 / 宽限期" }`)
	cpu, _ := runPS(`(Get-CimInstance Win32_Processor).Name`)
	gpu, _ := runPS(`(Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name) -join " / "`)
	board, _ := runPS(`$b = Get-CimInstance Win32_BaseBoard; "$($b.Manufacturer) $($b.Product)"`)
	net, _ := runPS(`(Get-CimInstance Win32_NetworkAdapter | Where-Object {$_.PhysicalAdapter -eq $true} | Select-Object -ExpandProperty Name) -join " / "`)
	missingRaw, _ := runPS(`Get-CimInstance Win32_PnPEntity | Where-Object { $_.ConfigManagerErrorCode -ne 0 } | Select-Object -ExpandProperty Name`)
	var missing []string
	for _, line := range strings.Split(missingRaw, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			missing = append(missing, t)
		}
	}
	role := "desktop"
	if strings.Contains(strings.ToLower(osVer), "server") {
		role = "server"
	}
	return &core.HardwareInfo{
		OSPlatform: "windows", OSVersion: osVer, OSRole: role, ActivationStatus: actStatus,
		CPU: cpu, GPU: gpu, Motherboard: board, NetworkCard: net, MissingDrivers: missing,
	}, nil
}

func (w *WindowsProvider) ActivateWithProductKey(key string) error {
	cleanKey := strings.TrimSpace(key)
	if !winKeyRe.MatchString(cleanKey) {
		return fmt.Errorf("产品密钥格式不合法，应为 25 位字母数字组合")
	}
	_, err := runPS(fmt.Sprintf(`cscript //nologo $env:SystemRoot\System32\slmgr.vbs /ipk %s; cscript //nologo $env:SystemRoot\System32\slmgr.vbs /ato`, cleanKey))
	return err
}

func (w *WindowsProvider) InstallOfficialDrivers(hw *core.HardwareInfo) error {
	_, err := runPS(`$s = New-Object -ComObject Microsoft.Update.Session; $r = $s.CreateUpdateSearcher().Search("IsInstalled=0 and Type='Driver'"); if ($r.Updates.Count -gt 0) { $d = $s.CreateUpdateDownloader(); $d.Updates = $r.Updates; $d.Download() | Out-Null; $i = $s.CreateUpdateInstaller(); $i.Updates = $r.Updates; $i.Install() | Out-Null }`)
	return err
}

func (w *WindowsProvider) ScanSystemPatches() ([]core.PatchItem, error) {
	script := `
		$s = New-Object -ComObject Microsoft.Update.Session
		$r = $s.CreateUpdateSearcher().Search("IsInstalled=0 and Type='Software' and IsHidden=0")
		$list = @()
		foreach ($u in $r.Updates) {
			$kb = if ($u.KBArticleIDs.Count -gt 0) { "KB" + $u.KBArticleIDs.Item(0) } else { "WIN-UPD" }
			$cat = if ($u.Title -match "Security|安全|Defender") { "Security" } elseif ($u.Title -match "Feature|功能更新") { "Feature" } else { "Quality" }
			$list += [PSCustomObject]@{ id = $kb; title = $u.Title; category = $cat }
		}
		if ($list.Count -eq 0) { "[]" } else { $list | ConvertTo-Json -Compress }
	`
	out, err := runPS(script)
	if err != nil || out == "" || out == "[]" {
		return nil, err
	}
	var items []core.PatchItem
	if strings.HasPrefix(out, "{") {
		var single core.PatchItem
		if json.Unmarshal([]byte(out), &single) == nil {
			items = append(items, single)
		}
		return items, nil
	}
	_ = json.Unmarshal([]byte(out), &items)
	return items, nil
}

func (w *WindowsProvider) InstallSystemPatches(securityOnly bool, logFn func(string)) (bool, error) {
	filter := `IsInstalled=0 and Type='Software' and IsHidden=0`
	if logFn != nil {
		if securityOnly {
			logFn("🛡️ 正在检索并安装 Windows 安全与质量补丁（已排除大版本功能更新，禁止自动重启）...")
		} else {
			logFn("🛡️ 正在检索并安装全部可用 Windows 系统补丁（禁止自动重启）...")
		}
	}
	skipFeature := "$false"
	if securityOnly {
		skipFeature = "$true"
	}
	script := fmt.Sprintf(`
		$s = New-Object -ComObject Microsoft.Update.Session
		$r = $s.CreateUpdateSearcher().Search("%s")
		$toInstall = New-Object -ComObject Microsoft.Update.UpdateColl
		foreach ($u in $r.Updates) {
			if (%s -and ($u.Title -match "Feature Update|功能更新")) { continue }
			$toInstall.Add($u) | Out-Null
		}
		if ($toInstall.Count -gt 0) {
			$d = $s.CreateUpdateDownloader(); $d.Updates = $toInstall; $d.Download() | Out-Null
			$i = $s.CreateUpdateInstaller(); $i.Updates = $toInstall; $res = $i.Install()
			if ($res.RebootRequired) { "REBOOT_REQUIRED" } else { "OK" }
		} else { "NONE" }
	`, filter, skipFeature)
	out, err := runPS(script)
	return strings.Contains(out, "REBOOT_REQUIRED"), err
}

func (w *WindowsProvider) OptimizeSystem(options []string) error {
	for _, opt := range options {
		if opt == "desk_perf" || opt == "high_perf" {
			_, _ = runPS(`powercfg /setactive 8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c`)
		} else if opt == "desk_telemetry" || opt == "disable_telemetry" {
			_, _ = runPS(`Set-ItemProperty -Path "HKLM:\SOFTWARE\Policies\Microsoft\Windows\DataCollection" -Name "AllowTelemetry" -Value 0 -Type DWord -Force -ErrorAction SilentlyContinue; Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced" -Name "HideFileExt" -Value 0 -Type DWord -Force`)
		}
	}
	return nil
}

func (w *WindowsProvider) InstallSystemRuntimes() error {
	_ = w.EnsureWinget(nil)
	for _, pkg := range []string{"Microsoft.VCRedist.2015+.x64", "Microsoft.VCRedist.2015+.x86", "Microsoft.DotNet.DesktopRuntime.8"} {
		cmd := exec.Command("winget", "install", "--id", pkg, "--silent", "--accept-package-agreements", "--accept-source-agreements")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
	}
	return nil
}

func (w *WindowsProvider) SearchSoftware(keyword string) (string, error) {
	if err := w.EnsureWinget(nil); err != nil {
		return "", err
	}
	safeKw := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' || r == ' ' {
			return r
		}
		return -1
	}, strings.TrimSpace(keyword))
	if safeKw == "" {
		return "", fmt.Errorf("搜索关键词不能为空")
	}
	return runPS(fmt.Sprintf(`winget search "%s" --accept-source-agreements | Select-Object -First 12`, safeKw))
}

func (w *WindowsProvider) InstallSoftware(item core.SoftwareItem, logFn func(string)) error {
	pkgID := strings.TrimSpace(item.Targets.Winget)
	if pkgID == "" {
		return nil
	}
	if item.BinName != "" {
		if _, err := exec.LookPath(item.BinName); err == nil {
			if logFn != nil {
				logFn(fmt.Sprintf("[已就绪] 检测到系统已存在 %s，跳过安装", item.BinName))
			}
			return nil
		}
	}
	if err := w.EnsureWinget(logFn); err != nil {
		return err
	}
	cmd := exec.Command("winget", "install", "--id", pkgID, "--silent", "--accept-package-agreements", "--accept-source-agreements")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); len(line) > 2 && logFn != nil {
			logFn(line)
		}
	}
	return cmd.Wait()
}

func (w *WindowsProvider) ConfigureDevEnvironment(item core.SoftwareItem, mirrorURL string) error {
	if mirrorURL == "official" || mirrorURL == "" {
		return nil
	}
	switch item.ID {
	case "go":
		_, _ = runPS(`setx GOPROXY "https://goproxy.cn,direct" /M`)
	case "python":
		_, _ = runPS(fmt.Sprintf(`pip config set global.index-url "%s/pypi/web/simple"`, strings.TrimRight(mirrorURL, "/")))
	case "nodejs":
		_, _ = runPS(`npm config set registry https://registry.npmmirror.com`)
	case "git":
		_, _ = runPS(`git config --global core.longpaths true; git config --global init.defaultBranch main`)
	}
	return nil
}

func (w *WindowsProvider) ApplyExtraConfig(dev core.DevEnvExtra, net core.NetworkExtra, role string, logFn func(string)) error {
	if h := strings.TrimSpace(net.CustomHostname); h != "" {
		_, _ = runPS(fmt.Sprintf(`Rename-Computer -NewName '%s' -Force -ErrorAction SilentlyContinue`, h))
	}
	if net.DisableHibernat {
		_, _ = runPS(`powercfg -h off`)
	}
	if dev.GoPrivateDomains != "" {
		_, _ = runPS(fmt.Sprintf(`setx GOPRIVATE "%s" /M`, strings.TrimSpace(dev.GoPrivateDomains)))
	}
	if dev.GitUserName != "" {
		_, _ = runPS(fmt.Sprintf(`git config --global user.name "%s"`, strings.TrimSpace(dev.GitUserName)))
	}
	if dev.GitUserEmail != "" {
		_, _ = runPS(fmt.Sprintf(`git config --global user.email "%s"`, strings.TrimSpace(dev.GitUserEmail)))
	}
	if dev.GitProxyPort != "" {
		_, _ = runPS(fmt.Sprintf(`git config --global http.proxy "http://127.0.0.1:%s"`, strings.TrimSpace(dev.GitProxyPort)))
	}
	if key := strings.TrimSpace(net.TailscaleKey); key != "" {
		tsCmd := fmt.Sprintf(`tailscale up --authkey=%s`, key)
		if r := strings.TrimSpace(net.TailscaleRoutes); r != "" {
			tsCmd += fmt.Sprintf(` --advertise-routes=%s`, r)
		}
		_, _ = runPS(tsCmd)
	}
	return nil
}

func (w *WindowsProvider) ExportInstalledSoftware() ([]core.SoftwareItem, error) {
	if err := w.EnsureWinget(nil); err != nil {
		return nil, err
	}
	tmpFile := filepath.Join(os.TempDir(), "sysinit_winget_export.json")
	defer os.Remove(tmpFile)
	cmd := exec.Command("winget", "export", "-o", tmpFile, "--accept-source-agreements")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Sources []struct {
			Packages []struct{ PackageIdentifier string `json:"PackageIdentifier"` } `json:"Packages"`
		} `json:"Sources"`
	}
	_ = json.Unmarshal(raw, &parsed)
	var items []core.SoftwareItem
	for _, src := range parsed.Sources {
		for _, pkg := range src.Packages {
			if pkg.PackageIdentifier == "" {
				continue
			}
			parts := strings.Split(pkg.PackageIdentifier, ".")
			items = append(items, core.SoftwareItem{
				ID: strings.ToLower(strings.ReplaceAll(pkg.PackageIdentifier, ".", "_")),
				Name: parts[len(parts)-1] + " (本机提取)", Category: "Tool", TargetRole: "desktop",
				Targets: core.PackageTargets{Winget: pkg.PackageIdentifier},
			})
			if len(items) >= 25 {
				break
			}
		}
	}
	return items, nil
}

func (w *WindowsProvider) VerifyEnvironment(items []core.SoftwareItem) []string {
	return core.CommonVerifyEnvironment(items)
}