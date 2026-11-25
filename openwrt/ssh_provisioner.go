package openwrt

import (
	"SysInitPro/sshx"
	"fmt"
	"strings"
)

type SSHProvisionRequest struct {
	Host        string `json:"host"`
	Port        string `json:"port"`
	User        string `json:"user"`
	Password    string `json:"password"`
	FeedSource  string `json:"feed_source"`
	BypassMode  bool   `json:"bypass_mode"`
	MainGateway string `json:"main_gateway"`
	InstallPkgs string `json:"install_pkgs"`
}

func RunSSHProvision(req SSHProvisionRequest) (string, error) {
	var cmds []string
	cmds = append(cmds, "uci set system.@system[0].zonename='Asia/Shanghai' && uci set system.@system[0].timezone='CST-8' && uci commit system")
	if req.FeedSource != "official" && req.FeedSource != "" {
		m := "mirrors.tuna.tsinghua.edu.cn/openwrt"
		if req.FeedSource == "ustc" {
			m = "mirrors.ustc.edu.cn/openwrt"
		} else if req.FeedSource == "aliyun" {
			m = "mirrors.aliyun.com/openwrt"
		}
		cmds = append(cmds, fmt.Sprintf("sed -i 's|downloads.openwrt.org|%s|g; s|downloads.immortalwrt.org|%s|g' /etc/opkg/distfeeds.conf 2>/dev/null || true", m, m))
	}
	if req.BypassMode && req.MainGateway != "" {
		cmds = append(cmds, fmt.Sprintf("uci set dhcp.lan.ignore='1' && uci set network.lan.gateway='%s' && uci add_list network.lan.dns='%s' && uci commit dhcp && uci commit network", req.MainGateway, req.MainGateway))
	}
	if pkgs := strings.TrimSpace(req.InstallPkgs); pkgs != "" {
		cmds = append(cmds, fmt.Sprintf("if command -v apk >/dev/null 2>&1; then apk update && apk add %s; else opkg update && opkg install %s; fi", pkgs, pkgs))
	}
	cmds = append(cmds, "/etc/init.d/network reload 2>/dev/null || true && echo '✅ OpenWrt 配置已生效！'")
	return sshx.RunCommand(sshx.SSHTarget{Host: req.Host, Port: req.Port, User: req.User, Password: req.Password}, strings.Join(cmds, " && "))
}