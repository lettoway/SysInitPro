package translator

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func ParseSingboxURI(rawURI string) (*SingboxFormConfig, error) {
	uri := strings.TrimSpace(rawURI)
	if uri == "" {
		return nil, fmt.Errorf("节点链接不能为空")
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("无法解析 URI 格式: %v", err)
	}
	cfg := &SingboxFormConfig{InboundMode: "mixed", MixedPort: 7890, BypassCN: true, EnableAIFilter: true, Hy2UpMbps: 100, Hy2DownMbps: 200, ServerPort: 443}
	switch strings.ToLower(u.Scheme) {
	case "vless":
		cfg.Protocol = "vless"
	case "hysteria2", "hy2":
		cfg.Protocol = "hysteria2"
	case "trojan":
		cfg.Protocol = "trojan"
	case "ss":
		cfg.Protocol = "shadowsocks"
	default:
		return nil, fmt.Errorf("暂不支持的协议头: %s", u.Scheme)
	}
	cfg.ServerAddr = u.Hostname()
	if p, err := strconv.Atoi(u.Port()); err == nil && p > 0 {
		cfg.ServerPort = p
	}
	if u.User != nil {
		userInfo := u.User.Username()
		if cfg.Protocol == "shadowsocks" && !strings.Contains(userInfo, ":") {
			if dec, err := base64.RawURLEncoding.DecodeString(userInfo); err == nil {
				parts := strings.SplitN(string(dec), ":", 2)
				if len(parts) == 2 {
					cfg.UUIDOrPass = parts[1]
				}
			} else {
				cfg.UUIDOrPass = userInfo
			}
		} else {
			cfg.UUIDOrPass = userInfo
		}
	}
	q := u.Query()
	if sni := q.Get("sni"); sni != "" {
		cfg.SNI = sni
	} else if peer := q.Get("peer"); peer != "" {
		cfg.SNI = peer
	}
	cfg.RealityPubKey, cfg.RealitySid, cfg.Flow = q.Get("pbk"), q.Get("sid"), q.Get("flow")
	return cfg, nil
}
