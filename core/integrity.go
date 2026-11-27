package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type IntegrityReport struct {
	OK             bool     `json:"ok"`
	StandaloneMode bool     `json:"standaloneMode"`
	Checked        int      `json:"checked"`
	Mismatched     []string `json:"mismatched"`
	Missing        []string `json:"missing"`
}

var criticalFiles = []string{
	"wails.json", "main.go", "app.go", "core/platform.go", "config/profile.go",
	"sshx/client.go", "pve/orchestrator.go", "translator/singbox_gen.go",
	"translator/vps_security.go", "internal/modules/openwrt/generator.go", "internal/modules/openwrt/builder.go",
	"platform/driver_windows.go", "frontend/index.html", "frontend/src/style.css", "frontend/src/main.js",
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyProjectIntegrity(projectRoot string) IntegrityReport {
	// 若当前目录不存在 go.mod，说明是独立发行的可执行文件，跳过源码目录校验
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err != nil {
		return IntegrityReport{OK: true, StandaloneMode: true}
	}

	rep := IntegrityReport{OK: true}
	for _, rel := range criticalFiles {
		full := filepath.Join(projectRoot, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil || info.Size() == 0 {
			rep.OK = false
			rep.Missing = append(rep.Missing, rel)
		}
		rep.Checked++
	}

	manifestPath := filepath.Join(projectRoot, "sysinit_manifest.sha256")
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return rep
	}

	for _, line := range strings.Split(string(rawManifest), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		expectedHash := strings.ToLower(parts[0])
		relPath := strings.Join(parts[1:], " ")
		full := filepath.Join(projectRoot, filepath.FromSlash(relPath))
		actualHash, fErr := sha256File(full)
		if fErr != nil {
			rep.OK = false
			if !containsStr(rep.Missing, relPath) {
				rep.Missing = append(rep.Missing, relPath)
			}
			continue
		}
		if actualHash != expectedHash {
			rep.OK = false
			rep.Mismatched = append(rep.Mismatched, relPath)
		}
	}
	return rep
}

func RefreshIntegrityManifest(projectRoot string) (int, error) {
	var entries []string
	err := filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil || rel == "." {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		if slashRel == "dist" || strings.HasPrefix(slashRel, "dist/") ||
			slashRel == "build" || strings.HasPrefix(slashRel, "build/") ||
			slashRel == ".git" || strings.HasPrefix(slashRel, ".git/") ||
			slashRel == "sysinit_manifest.sha256" || slashRel == "apply_v901.ps1" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		sum, err := sha256File(path)
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s  %s", sum, slashRel))
		return nil
	})
	if err != nil {
		return 0, err
	}
	sort.Strings(entries)
	var sb strings.Builder
	sb.WriteString("# SysInitPro v9.0.1 SHA-256 Integrity Manifest\n")
	for _, e := range entries {
		sb.WriteString(e + "\n")
	}
	manifestPath := filepath.Join(projectRoot, "sysinit_manifest.sha256")
	if err := os.WriteFile(manifestPath, []byte(sb.String()), 0644); err != nil {
		return 0, err
	}
	return len(entries), nil
}

func containsStr(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func FormatIntegritySummary(rep IntegrityReport) string {
	if rep.StandaloneMode {
		return "[OK] 当前为独立发行版运行模式，已自动跳过开发源码目录校验。"
	}
	if rep.OK {
		return fmt.Sprintf("[OK] 代码完整性校验通过（已核验 %d 项核心文件与哈希清单，无缺失或篡改）", rep.Checked)
	}
	var b strings.Builder
	b.WriteString("[FAIL] 代码完整性校验未通过：\n")
	for _, m := range rep.Missing {
		b.WriteString(fmt.Sprintf("  - 缺失或空文件: %s\n", m))
	}
	for _, diff := range rep.Mismatched {
		b.WriteString(fmt.Sprintf("  - 哈希不匹配: %s\n", diff))
	}
	return b.String()
}
