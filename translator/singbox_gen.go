package translator

import (
	"SysInitPro/sshx"
	"encoding/json"
	"fmt"
	"strings"
)

type SingboxFormConfig struct {
	InboundMode    string `json:"inbound_mode"`
	MixedPort      int    `json:"mixed_port"`
	AllowLan       bool   `json:"allow_lan"`
	Protocol       string `json:"protocol"`
	ServerAddr     string `json:"server_addr"`
	ServerPort     int    `json:"server_port"`
	UUIDOrPass     string `json:"uuid_or_pass"`
	Flow           string `json:"flow"`
	SNI            string `json:"sni"`
	RealityPubKey  string `json:"reality_pubkey"`
	RealitySid     string `json:"reality_sid"`
	Hy2UpMbps      int    `json:"hy2_up_mbps"`
	Hy2DownMbps    int    `json:"hy2_down_mbps"`
	BypassCN       bool   `json:"bypass_cn"`
	EnableAIFilter bool   `json:"enable_ai_filter"`
	UseWarpForAI   bool   `json:"use_warp_for_ai"`
}

func GenerateSingboxJSON(cfg SingboxFormConfig) (string, error) {
	if cfg.MixedPort <= 0 {
		cfg.MixedPort = 7890
	}
	if cfg.ServerPort <= 0 {
		cfg.ServerPort = 443
	}
	listenAddr := "127.0.0.1"
	if cfg.AllowLan {
		listenAddr = "0.0.0.0"
	}
	inbounds := []map[string]interface{}{
		{"type": "mixed", "tag": "mixed-in", "listen": listenAddr, "listen_port": cfg.MixedPort, "sniff": true, "sniff_override_destination": true},
	}
	if cfg.InboundMode == "tun" {
		inbounds = append(inbounds, map[string]interface{}{"type": "tun", "tag": "tun-in", "inet4_address": "172.19.0.1/30", "auto_route": true, "strict_route": true, "stack": "system", "sniff": true})
	}
	proxyOut := map[string]interface{}{"tag": "proxy", "server": strings.TrimSpace(cfg.ServerAddr), "server_port": cfg.ServerPort}
	switch cfg.Protocol {
	case "hysteria2":
		proxyOut["type"], proxyOut["password"] = "hysteria2", strings.TrimSpace(cfg.UUIDOrPass)
		if cfg.Hy2UpMbps > 0 {
			proxyOut["up_mbps"] = cfg.Hy2UpMbps
		}
		if cfg.Hy2DownMbps > 0 {
			proxyOut["down_mbps"] = cfg.Hy2DownMbps
		}
		proxyOut["tls"] = map[string]interface{}{"enabled": true, "server_name": cfg.SNI}
	case "vless":
		proxyOut["type"], proxyOut["uuid"] = "vless", strings.TrimSpace(cfg.UUIDOrPass)
		if cfg.Flow != "" {
			proxyOut["flow"] = cfg.Flow
		}
		tlsObj := map[string]interface{}{"enabled": true, "server_name": cfg.SNI, "utls": map[string]interface{}{"enabled": true, "fingerprint": "chrome"}}
		if strings.TrimSpace(cfg.RealityPubKey) != "" {
			tlsObj["reality"] = map[string]interface{}{"enabled": true, "public_key": strings.TrimSpace(cfg.RealityPubKey), "short_id": strings.TrimSpace(cfg.RealitySid)}
		}
		proxyOut["tls"] = tlsObj
	case "trojan":
		proxyOut["type"], proxyOut["password"] = "trojan", strings.TrimSpace(cfg.UUIDOrPass)
		proxyOut["tls"] = map[string]interface{}{"enabled": true, "server_name": cfg.SNI}
	default:
		proxyOut["type"], proxyOut["method"], proxyOut["password"] = "shadowsocks", "2022-blake3-aes-128-gcm", strings.TrimSpace(cfg.UUIDOrPass)
	}

	outbounds := []map[string]interface{}{proxyOut, {"type": "direct", "tag": "direct"}, {"type": "block", "tag": "block"}, {"type": "dns", "tag": "dns-out"}}
	aiTarget := "proxy"
	if cfg.UseWarpForAI {
		outbounds = append(outbounds, map[string]interface{}{"type": "socks", "tag": "warp-out", "server": "127.0.0.1", "server_port": 40000, "version": "5"})
		aiTarget = "warp-out"
	}

	rules := []map[string]interface{}{{"protocol": "dns", "outbound": "dns-out"}, {"ip_is_private": true, "outbound": "direct"}}
	if cfg.EnableAIFilter {
		rules = append(rules, map[string]interface{}{
			"domain_suffix": []string{"openai.com", "chatgpt.com", "anthropic.com", "claude.ai", "gemini.google.com", "aistudio.google.com", "googleapis.com", "x.ai", "grok.com", "cursor.com", "youtube.com"},
			"outbound":      aiTarget,
		})
	}
	routeObj := map[string]interface{}{"rules": rules, "final": "proxy", "auto_detect_interface": true}
	if cfg.BypassCN {
		rules = append(rules, map[string]interface{}{"rule_set": []string{"geosite-cn", "geoip-cn"}, "outbound": "direct"})
		routeObj["rules"] = rules
		routeObj["rule_set"] = []map[string]interface{}{
			{"tag": "geosite-cn", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs", "download_detour": "proxy"},
			{"tag": "geoip-cn", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs", "download_detour": "proxy"},
		}
	}

	fullConfig := map[string]interface{}{
		"log":       map[string]interface{}{"level": "info", "timestamp": true},
		"dns":       map[string]interface{}{"servers": []map[string]interface{}{{"tag": "remote-dns", "address": "https://1.1.1.1/dns-query", "detour": "proxy"}, {"tag": "local-dns", "address": "223.5.5.5", "detour": "direct"}}, "rules": []map[string]interface{}{{"outbound": "any", "server": "local-dns"}}},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route":     routeObj,
	}
	raw, err := json.MarshalIndent(fullConfig, "", "  ")
	return string(raw), err
}

func DeploySingboxToVPS(t sshx.SSHTarget, sbCfg SingboxFormConfig) (string, error) {
	jsonStr, err := GenerateSingboxJSON(sbCfg)
	if err != nil {
		return "", err
	}
	port := sbCfg.MixedPort
	if port <= 0 {
		port = 7890
	}
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -e
command -v sing-box >/dev/null 2>&1 || curl -fsSL https://sing-box.app/install.sh | sh
mkdir -p /etc/sing-box
cat << 'SYSEOF' > /etc/sing-box/config.json
%s
SYSEOF
sing-box check -c /etc/sing-box/config.json
command -v ufw >/dev/null 2>&1 && ufw allow %d/tcp || true
systemctl enable sing-box && systemctl restart sing-box
echo '✅ Sing-box 已成功部署并启动！'
`, jsonStr, port)
	return sshx.RunScript(t, script, true)
}