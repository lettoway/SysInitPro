package openwrt

import (
	"SysInitPro/internal/models"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConflictWarning struct {
	Level   string   `json:"level"`
	Title   string   `json:"title"`
	Message string   `json:"message"`
	Plugins []string `json:"plugins"`
}

type ConflictRule struct {
	Type        string   `json:"type"`  // "mutex_group" | "pair_mutex" | "branch_risk"
	Level       string   `json:"level"` // "warn" | "error"
	Title       string   `json:"title"`
	Message     string   `json:"message"`
	Plugins     []string `json:"plugins"`
	BranchMatch string   `json:"branchMatch,omitempty"` // 仅 branch_risk 使用
}

var proxyPlugins = map[string]bool{
	"luci-app-passwall":  true,
	"luci-app-passwall2": true,
	"luci-app-openclash": true,
	"luci-app-homeproxy": true,
	"luci-app-ssr-plus":  true,
	"luci-app-vssr":      true,
	"luci-app-bypass":    true,
}

func defaultConflictRules() []ConflictRule {
	return []ConflictRule{
		{
			Type:    "mutex_group",
			Level:   "warn",
			Title:   "代理插件重叠",
			Message: "同时选择了 %s，易引发 sing-box/xray 依赖冲突或抢占 DNS 53 端口，建议仅保留 1 个。",
			Plugins: []string{"luci-app-passwall", "luci-app-passwall2", "luci-app-openclash", "luci-app-homeproxy", "luci-app-ssr-plus", "luci-app-vssr", "luci-app-bypass"},
		},
		{
			Type:    "pair_mutex",
			Level:   "warn",
			Title:   "文件共享端口冲突",
			Message: "luci-app-samba4 与 luci-app-ksmbd 同时占用 445 端口且显著增加固件体积，建议二选一。",
			Plugins: []string{"luci-app-samba4", "luci-app-ksmbd"},
		},
		{
			Type:        "branch_risk",
			Level:       "error",
			Title:       "24.10 防火墙兼容风险 (fw4)",
			Message:     "24.10 默认使用 nftables (fw4)，%s 可能依赖旧版 iptables 补丁导致编译报错，请确认对应源已适配 24.10。",
			Plugins:     []string{"luci-app-turboacc", "luci-app-flowoffload", "luci-app-ssr-plus"},
			BranchMatch: "24.10",
		},
		{
			Type:    "pair_mutex",
			Level:   "warn",
			Title:   "Web 服务端口冲突",
			Message: "nginx 与 uhttpd 默认均监听 80/443 端口，请确保仅启用一个或手动修改端口。",
			Plugins: []string{"luci-app-nginx", "luci-app-uhttpd"},
		},
	}
}

func loadConflictRules() []ConflictRule {
	dir, err := os.UserConfigDir()
	if err != nil {
		return defaultConflictRules()
	}
	p := filepath.Join(dir, "SysInitPro", "conflict_rules.json")
	if raw, err := os.ReadFile(p); err == nil {
		var rules []ConflictRule
		if json.Unmarshal(raw, &rules) == nil && len(rules) > 0 {
			return rules
		}
	}
	// 首次自动写出默认规则文件，方便用户随时自定义
	rules := defaultConflictRules()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	if b, err := json.MarshalIndent(rules, "", "  "); err == nil {
		_ = os.WriteFile(p, b, 0644)
	}
	return rules
}

func CheckConflicts(cfg models.FirmwareConfig) []ConflictWarning {
	selected := make(map[string]bool, len(cfg.Plugins))
	for _, p := range cfg.Plugins {
		selected[p] = true
	}

	var list []ConflictWarning
	for _, r := range loadConflictRules() {
		var hit []string
		for _, p := range r.Plugins {
			if selected[p] {
				hit = append(hit, p)
			}
		}
		switch r.Type {
		case "mutex_group":
			if len(hit) >= 2 {
				msg := r.Message
				if strings.Contains(msg, "%s") {
					msg = fmt.Sprintf(msg, strings.Join(hit, " / "))
				}
				list = append(list, ConflictWarning{Level: r.Level, Title: r.Title, Message: msg, Plugins: hit})
			}
		case "pair_mutex":
			if len(hit) == len(r.Plugins) && len(hit) >= 2 {
				list = append(list, ConflictWarning{Level: r.Level, Title: r.Title, Message: r.Message, Plugins: hit})
			}
		case "branch_risk":
			if r.BranchMatch != "" && strings.Contains(cfg.Branch, r.BranchMatch) && len(hit) > 0 {
				msg := r.Message
				if strings.Contains(msg, "%s") {
					msg = fmt.Sprintf(msg, strings.Join(hit, " / "))
				}
				list = append(list, ConflictWarning{Level: r.Level, Title: r.Title, Message: msg, Plugins: hit})
			}
		}
	}
	return list
}

func BuildBundle(cfg models.FirmwareConfig) models.FirmwareBundle {
	b := buildBundleRaw(cfg)

	selected := make(map[string]bool, len(cfg.Plugins))
	hasProxy := false
	for _, p := range cfg.Plugins {
		selected[p] = true
		if proxyPlugins[p] {
			hasProxy = true
		}
	}

	var autoNotes []string
	if hasProxy && !strings.Contains(b.DotConfig, "CONFIG_PACKAGE_dnsmasq-full=y") {
		b.DotConfig += "\n# [SysInitPro 自动消解] 代理插件需替换为 dnsmasq-full\n# CONFIG_PACKAGE_dnsmasq is not set\nCONFIG_PACKAGE_dnsmasq-full=y\n"
		autoNotes = append(autoNotes, "已自动切换 dnsmasq -> dnsmasq-full")
	}
	if selected["luci-app-ksmbd"] && !selected["luci-app-samba4"] && !strings.Contains(b.DotConfig, "CONFIG_PACKAGE_luci-app-samba4 is not set") {
		b.DotConfig += "\n# [SysInitPro 自动消解] 启用 KSMBD 时禁用 Samba4\n# CONFIG_PACKAGE_luci-app-samba4 is not set\n"
		autoNotes = append(autoNotes, "已自动屏蔽 samba4")
	}
	if len(autoNotes) > 0 {
		b.DotConfig = "# 自动依赖消解: " + strings.Join(autoNotes, "；") + "\n" + b.DotConfig
	}

	defCheck := `make defconfig 2>&1 | tee defconfig.log
          if grep -Eq "conflicts with|Recursive dependency" defconfig.log; then
            echo "::error::检测到致命插件依赖冲突，已中止编译："
            grep -E "conflicts with|Recursive dependency" defconfig.log
            exit 1
          fi`
	if strings.Contains(b.WorkflowYml, "make defconfig") && !strings.Contains(b.WorkflowYml, "defconfig.log") {
		b.WorkflowYml = strings.Replace(b.WorkflowYml, "make defconfig", defCheck, 1)
	}
	return b
}
