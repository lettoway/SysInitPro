package openwrt

import (
	"SysInitPro/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PluginI18nEntry struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

var builtinPluginI18n = map[string]PluginI18nEntry{
	// --- 代理 / 分流 / DNS ---
	"luci-app-passwall":    {"PassWall 科学代理", "轻量级多协议科学上网与分流工具，支持 Xray/Sing-box"},
	"luci-app-passwall2":   {"PassWall 2 代理", "基于 Xray/Sing-box 分流引擎的新一代 PassWall"},
	"luci-app-openclash":   {"OpenClash 代理", "Clash/Mihomo 内核客户端，支持精细规则分流与覆写"},
	"luci-app-homeproxy":   {"HomeProxy 代理", "ImmortalWrt 官方维护的原生 Sing-box 代理客户端"},
	"luci-app-ssr-plus":    {"ShadowSocksR Plus+", "经典老牌多协议科学上网插件，界面简洁"},
	"luci-app-vssr":        {"Hello World (VSSR)", "可视化国旗节点界面的多协议代理客户端"},
	"luci-app-bypass":      {"Bypass 科学代理", "简洁高效的多协议透明代理与分流工具"},
	"luci-app-mosdns":      {"MosDNS 分流防污染", "高性能插件化 DNS 转发器，精准国内外域名分流"},
	"luci-app-adguardhome": {"AdGuard Home 去广告", "全网 DNS 级广告拦截、隐私保护与查询日志审计"},
	"luci-app-smartdns":    {"SmartDNS 智能解析", "多上游并发测速返回最快 IP，显著加速网页打开"},
	"luci-app-chinadns-ng": {"ChinaDNS-NG 分流", "轻量级国内外 DNS 分流防污染守护进程"},
	"luci-app-dnsfilter":   {"DNSFilter 广告过滤", "基于域名规则表的轻量级 DNS 广告拦截"},
	"luci-app-adbyby-plus": {"广告屏蔽大师 Plus+", "老牌规则过滤去广告插件，支持多种订阅规则"},

	// --- 内网穿透 / 异地组网 / VPN ---
	"luci-app-frpc":           {"FRP 内网穿透客户端", "通过公网中转服务器安全暴露内网 Web/SSH 服务"},
	"luci-app-frps":           {"FRP 内网穿透服务端", "在具备公网 IP 的路由器上搭建 FRP 中转服务端"},
	"luci-app-zerotier":       {"ZeroTier 异地组网", "零配置虚拟局域网，轻松实现跨地域内网设备直连"},
	"luci-app-tailscale":      {"Tailscale 异地组网", "基于 WireGuard 的现代 P2P 异地组网与出口节点"},
	"luci-app-wireguard":      {"WireGuard VPN", "内核级高性能现代 VPN 协议，低延迟省资源"},
	"luci-app-openvpn":        {"OpenVPN 客户端/服务", "传统 SSL/TLS 虚拟专用网络隧道管理"},
	"luci-app-openvpn-server": {"OpenVPN 服务端", "一键生成客户端证书，快速搭建家庭回连 VPN"},
	"luci-app-ddns":           {"动态域名 DDNS", "自动将动态公网 IP 绑定到域名，支持主流 DNS 服务商"},
	"luci-app-ddns-go":        {"DDNS-Go 动态域名", "现代 Web 界面的动态域名工具，原生支持 IPv4/IPv6"},
	"luci-app-ddnsto":         {"DDNSTO 远程控制", "酷友社出品的免公网 IP 远程桌面与路由管理穿透"},
	"luci-app-nps":            {"NPS 内网穿透", "轻量级高性能内网穿透代理客户端"},
	"luci-app-n2n":            {"N2N P2P 组网", "去中心化二层 P2P 虚拟局域网组网工具"},
	"luci-app-ipsec-vpnd":     {"IPSec VPN 服务端", "免客户端原生兼容 iOS/macOS/Windows 的 IPSec 服务端"},
	"luci-app-socat":          {"Socat 端口转发", "强大的 IPv4/IPv6 TCP/UDP 端口映射与流量转发"},
	"luci-app-upnp":           {"UPnP 自动端口映射", "允许局域网游戏机/BT 软件自动申请公网端口映射"},

	// --- 网络优化 / 负载均衡 / 防火墙 ---
	"luci-app-mwan3":         {"MWAN3 多线负载", "多宽带叠加、策略路由分流与断线自动故障转移"},
	"luci-app-sqm":           {"SQM 智能流控", "基于 CAKE/FQ_CoDel 消除缓冲膨胀，降低游戏延迟"},
	"luci-app-qos":           {"QoS 服务质量", "按 IP/端口分配上下行带宽优先级"},
	"luci-app-turboacc":      {"Turbo ACC 网络加速", "集成软件/硬件流量卸载与 BBR 拥塞控制加速"},
	"luci-app-flowoffload":   {"Flow Offload 加速", "内核级转发加速模块，降低高吞吐时 CPU 占用"},
	"luci-app-firewall":      {"防火墙管理", "端口转发、通信规则、区域策略与 NAT 配置"},
	"luci-app-banip":         {"BanIP 恶意拦截", "按黑名单/国家 IP 段自动封禁恶意扫描与攻击"},
	"luci-app-bcp38":         {"BCP38 防源地址欺骗", "阻止内网私网地址段流量泄露到公网"},
	"luci-app-wol":           {"网络唤醒 (WoL)", "通过局域网魔术包远程唤醒关机状态的电脑/NAS"},
	"luci-app-etherwake":     {"EtherWake 远程唤醒", "轻量级局域网 MAC 魔术包开机工具"},
	"luci-app-vlmcsd":        {"KMS 激活服务器", "局域网内自动激活 Windows 与 Office 批量授权版"},
	"luci-app-arpbind":       {"IP/MAC 静态绑定", "防止局域网 ARP 欺骗，固定设备内网地址"},
	"luci-app-autoreboot":    {"定时重启计划", "按周期自动重启路由器以释放内存保持稳定"},
	"luci-app-accesscontrol": {"上网时间控制", "按 MAC 地址和时段限制指定设备访问互联网"},
	"luci-app-eqos":          {"EQoS 按 IP 限速", "简单直观地为局域网单个 IP 设置上传/下载限速"},
	"luci-app-nft-qos":       {"NFT-QoS 限速", "基于 nftables 的新版按 IP/MAC 流量限速与优先级"},
	"luci-app-syncdial":      {"多拨虚拟网卡", "单线多拨叠加宽带速率（需配合 MWAN3 使用）"},

	// --- 存储 / NAS / 下载 / 文件共享 ---
	"luci-app-samba4":        {"Samba4 文件共享", "经典 SMB/CIFS 局域网共享，兼容性好，支持 TimeMachine"},
	"luci-app-ksmbd":         {"KSMBD 内核共享", "Linux 内核态 SMB 服务，体积小、读写性能高于 Samba4"},
	"luci-app-aria2":         {"Aria2 离线下载", "支持 HTTP/BT/磁力链的多线程后台下载器"},
	"luci-app-transmission":  {"Transmission BT 下载", "轻量稳定的 BitTorrent / PT 挂机做种客户端"},
	"luci-app-qbittorrent":   {"qBittorrent 下载", "功能丰富的 BT/PT 下载器，支持 RSS 自动订阅"},
	"luci-app-alist":         {"AList 网盘聚合", "将阿里云盘/夸克/OneDrive 等挂载为本地 WebDAV"},
	"luci-app-openlist":      {"OpenList 网盘聚合", "AList 社区开源分支，支持多网盘聚合与 WebDAV"},
	"luci-app-cloudreve":     {"Cloudreve 私有云盘", "支持多存储策略的自建私有网盘系统"},
	"luci-app-minidlna":      {"MiniDLNA 媒体共享", "向局域网电视、投影仪推送本地影音文件"},
	"luci-app-hd-idle":       {"硬盘自动休眠", "设定外接机械硬盘空闲自动停转，降噪省电护盘"},
	"luci-app-diskman":       {"DiskMan 磁盘管理", "可视化硬盘分区、格式化、挂载与 SMART 健康检测"},
	"luci-app-cifs-mount":    {"CIFS 远程挂载", "将局域网其他 NAS 的 SMB 共享目录挂载到本机"},
	"luci-app-nfs":           {"NFS 网络文件系统", "面向 Linux/PVE/播放器的高性能局域网文件共享"},
	"luci-app-webdav":        {"WebDAV 文件服务", "通过 HTTP/HTTPS 协议提供跨平台文件挂载访问"},
	"luci-app-filebrowser":   {"FileBrowser 文件管理", "现代 Web 界面的在线文件浏览、上传与分享工具"},
	"luci-app-fileassistant": {"文件助手", "在 LuCI 后台直接管理、编辑和上传路由器本地文件"},
	"luci-app-linkease":      {"易有云文件同步", "跨设备文件备份、相册同步与远程文件访问"},
	"luci-app-baidupcs-web":  {"百度网盘 Web 版", "在路由器后台直接高速下载/管理百度网盘文件"},
	"luci-app-rclone":        {"Rclone 云存储同步", "支持数十种对象存储与网盘的挂载和定时同步"},

	// --- 容器 / 虚拟化 / 商店 ---
	"luci-app-dockerman":  {"Docker 容器管理", "可视化管理 Docker 容器、镜像、网络与数据卷"},
	"luci-app-store":      {"iStore 软件中心", "图形化 OpenWrt 应用商店，支持一键安装与卸载插件"},
	"luci-app-quickstart": {"QuickStart 首页导航", "iStoreOS 风格的可视化路由状态首页与向导"},
	"luci-app-istorex":    {"iStoreX 增强首页", "提供更直观的系统概览与快捷配置入口"},
	"luci-app-lxc":        {"LXC 轻量容器", "在路由器上运行轻量级 Linux 系统容器"},

	// --- 系统监控 / 终端 / 工具 ---
	"luci-app-ttyd":                {"TTYD 网页终端", "在浏览器中直接打开路由器 SSH 命令行终端"},
	"luci-app-nlbwmon":             {"局域网流量统计", "按终端设备统计每日/每月上传下载流量明细"},
	"luci-app-statistics":          {"Collectd 性能图表", "记录并绘制 CPU、内存、负载与网卡流量历史曲线"},
	"luci-app-netdata":             {"Netdata 实时监控", "高刷新率可视化系统资源与网络吞吐监控面板"},
	"luci-app-watchcat":            {"Watchcat 断网自愈", "检测网络长时间中断时自动重启接口或整机"},
	"luci-app-commands":            {"自定义快捷命令", "在网页后台一键执行预设的 Shell 脚本命令"},
	"luci-app-attendedsysupgrade":  {"在线固件升级", "自动请求云端构建包含当前已装插件的新版固件"},
	"luci-app-advanced-reboot":     {"双分区切换重启", "支持双分区路由器在两个固件分区之间自由切换"},
	"luci-app-ramfree":             {"一键释放内存", "清理 Linux 页面缓存，快速释放路由器可用内存"},
	"luci-app-cpufreq":             {"CPU 频率调节", "自定义处理器调频策略，平衡性能与功耗发热"},
	"luci-app-temp-status":         {"硬件温度监控", "实时显示 CPU、主板与无线网卡传感器温度"},
	"luci-app-smartinfo":           {"SMART 硬盘健康", "监测 SATA/NVMe 硬盘温度、通电时间与坏道预警"},
	"luci-app-uhttpd":              {"uHTTPd Web 服务", "OpenWrt 默认轻量级后台管理 Web 服务器配置"},
	"luci-app-nginx":               {"Nginx 反向代理", "高性能 Web 服务器与 HTTPS 反向代理管理"},
	"luci-app-acme":                {"ACME 自动 SSL 证书", "自动申请并续期 Let's Encrypt 免费 HTTPS 证书"},
	"luci-app-unblockneteasemusic": {"解锁网易云灰色歌曲", "自动替换音源解锁网易云音乐变灰与无版权曲目"},
	"luci-app-serverchan":          {"微信/Server酱推送", "设备上下线、IP 变动与系统告警实时推送到手机"},
	"luci-app-wechatpush":          {"微信推送通知", "支持企业微信/钉钉/飞书的路由器状态与登录通知"},
	"luci-app-pushbot":             {"PushBot 全能推送", "多渠道路由器运行状态与新设备接入实时提醒"},
	"luci-app-udpxy":               {"UDPXY 组播转单播", "将 IPTV 组播流转为 HTTP 单播，方便手机/电脑看电视"},
	"luci-app-igmpproxy":           {"IGMP 组播代理", "转发宽带光猫 IPTV 组播流量到局域网机顶盒"},
	"luci-app-usb-printer":         {"USB 打印机共享", "将普通 USB 打印机接入路由器变成全屋网络打印机"},
	"luci-app-p910nd":              {"p910nd 打印服务器", "极简低占用的 USB/并口网络打印共享守护进程"},
	"luci-app-k3screenctrl":        {"K3 屏幕控制", "适配斐讯 K3 路由器的机身液晶屏显示与休眠控制"},
	"luci-app-oled":                {"OLED 屏幕驱动", "驱动软路由/开发板外接 I2C OLED 状态显示屏"},
	"luci-app-timecontrol":         {"上网时间管控", "灵活设置家庭终端允许上网的时间段规则"},
	"luci-app-parentcontrol":       {"家长控制", "按时间段和关键词过滤保护儿童健康上网"},
	"luci-app-wifischedule":        {"Wi-Fi 定时开关", "按时间表自动关闭或开启无线网络，节能防干扰"},
	"luci-app-guest-wifi":          {"访客 Wi-Fi 隔离", "一键创建与主内网隔离的访客专用无线网络"},
	"luci-app-travelmate":          {"Travelmate 无线中继", "便携路由器自动连接酒店/咖啡馆热点并处理认证页"},
	"luci-app-easymesh":            {"EasyMesh 无线漫游", "多台 OpenWrt 路由器组建无缝漫游 Mesh 网络"},
	"luci-app-dawn":                {"DAWN 漫游辅助", "基于 802.11k/v 优化多 AP 间的无线终端切换体验"},
	"luci-app-usteer":              {"uSteer 无线引导", "轻量级多 AP 负载均衡与弱信号自动切换"},
	"luci-app-nut":                 {"NUT UPS 电源监控", "监控外接 UPS 电量并在市电断电时安全关机"},
	"luci-app-apcupsd":             {"APC UPS 管理", "适配 APC 品牌不间断电源的状态监控与断电保护"},
	"luci-app-homeassistant":       {"Home Assistant", "开源智能家居中枢平台集成入口"},
	"luci-app-mosquitto":           {"Mosquitto MQTT", "轻量级物联网 MQTT 消息代理服务器"},
	"luci-app-mjpg-streamer":       {"摄像头视频流", "将 USB 摄像头画面转为局域网实时监控视频流"},
	"luci-app-vnstat2":             {"vnStat 流量统计", "长期记录 WAN/LAN 接口的日/月/年历史总流量"},
	"luci-app-iperf3-server":       {"iPerf3 测速服务端", "在路由器开启内网吞吐测速服务，排查网线/Wi-Fi 瓶颈"},
	"luci-app-netspeedtest":        {"网络测速工具", "在路由器后台一键测试宽带外网上下行速率与延迟"},
	"luci-app-diag-core":           {"网络诊断工具", "集成 Ping、Traceroute、Nslookup 等常用排障工具"},
}

func loadUserPluginI18n() map[string]PluginI18nEntry {
	out := map[string]PluginI18nEntry{}
	dir, err := os.UserConfigDir()
	if err != nil {
		return out
	}
	p := filepath.Join(dir, "SysInitPro", "plugin_i18n.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func feedDisplayLabel(feed string) string {
	switch feed {
	case "official":
		return "官方源扩展插件"
	case "kenzok8":
		return "KenzoK8 第三方扩展插件"
	case "istore":
		return "iStore 生态扩展插件"
	default:
		return "扩展插件"
	}
}

// EnrichPlugins 为插件列表补全中文名与中文说明（内置词典 + 用户自定义覆盖 + 智能兜底）
func EnrichPlugins(items []models.PluginItem) []models.PluginItem {
	userDict := loadUserPluginI18n()
	res := make([]models.PluginItem, len(items))
	for i, item := range items {
		short := strings.TrimPrefix(item.ID, "luci-app-")
		if u, ok := userDict[item.ID]; ok {
			if strings.TrimSpace(u.Name) != "" {
				item.Name = u.Name
			}
			if strings.TrimSpace(u.Desc) != "" {
				item.Desc = u.Desc
			}
		} else if b, ok := builtinPluginI18n[item.ID]; ok {
			item.Name = b.Name
			item.Desc = b.Desc
		} else {
			if item.Name == "" || item.Name == item.ID {
				item.Name = short
			}
			if item.Desc == "" || item.Desc == item.ID || item.Desc == short {
				item.Desc = feedDisplayLabel(item.Feed)
			}
		}
		res[i] = item
	}
	return res
}
func saveUserPluginI18n(dict map[string]PluginI18nEntry) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	appDir := filepath.Join(dir, "SysInitPro")
	_ = os.MkdirAll(appDir, 0755)
	b, err := json.MarshalIndent(dict, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(appDir, "plugin_i18n.json"), b, 0644)
}

func translateEnToZh(client *http.Client, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	u := "https://translate.googleapis.com/translate_a/single?client=gtx&sl=en&tl=zh-CN&dt=t&q=" + url.QueryEscape(text)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return ""
	}
	var raw []interface{}
	if json.Unmarshal(body, &raw) != nil || len(raw) == 0 {
		return ""
	}
	segs, ok := raw[0].([]interface{})
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, s := range segs {
		if arr, ok := s.([]interface{}); ok && len(arr) > 0 {
			if part, ok := arr[0].(string); ok {
				sb.WriteString(part)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

func smartKeywordZh(id, feed string) PluginI18nEntry {
	short := strings.TrimPrefix(id, "luci-app-")
	low := strings.ToLower(short)
	rules := []struct {
		kw   string
		name string
		desc string
	}{
		{"vpn", "VPN 隧道扩展", "虚拟专用网络客户端或服务端管理插件"},
		{"dns", "DNS 解析扩展", "域名解析、转发或防污染相关管理插件"},
		{"proxy", "代理服务扩展", "网络代理与流量转发管理插件"},
		{"wifi", "无线网络扩展", "Wi-Fi 无线配置与信号管理插件"},
		{"fw", "防火墙扩展", "防火墙规则与网络安全管理插件"},
		{"usb", "USB 设备扩展", "USB 外设驱动与共享管理插件"},
		{"smb", "文件共享扩展", "局域网 SMB 文件共享管理插件"},
		{"net", "网络工具扩展", "网络诊断、监控或接口管理插件"},
		{"stat", "状态监控扩展", "系统运行状态与流量统计插件"},
		{"led", "指示灯控制", "路由器机身 LED 指示灯触发规则管理"},
		{"tiny", "轻量代理扩展", "轻量级网络转发与代理管理插件"},
	}
	for _, r := range rules {
		if strings.Contains(low, r.kw) {
			return PluginI18nEntry{
				Name: r.name + " (" + short + ")",
				Desc: r.desc + "（" + feedDisplayLabel(feed) + "）",
			}
		}
	}
	return PluginI18nEntry{
		Name: "扩展组件 (" + short + ")",
		Desc: feedDisplayLabel(feed) + "，提供 " + short + " 相关 Web 管理界面",
	}
}

// TranslateMissingPlugins 批量补全缺失插件的中文名与说明（2.5秒连通性熔断 + 6路并发）
func TranslateMissingPlugins(items []models.PluginItem) ([]models.PluginItem, string) {
	userDict := loadUserPluginI18n()

	// 1. 筛选待补全项
	var todo []models.PluginItem
	for _, item := range items {
		if _, ok := builtinPluginI18n[item.ID]; ok {
			continue
		}
		if u, ok := userDict[item.ID]; ok && u.Name != "" && u.Desc != "" {
			continue
		}
		todo = append(todo, item)
	}
	if len(todo) == 0 {
		return EnrichPlugins(items), "所有插件均已有中文名与说明，无需重复补全"
	}

	// 2. 快速探测在线翻译连通性（2.5 秒超时熔断）
	probeClient := &http.Client{Timeout: 2500 * time.Millisecond}
	onlineOK := translateEnToZh(probeClient, "network") != ""

	client := &http.Client{Timeout: 4 * time.Second}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	translatedCount := 0
	fallbackCount := 0

	for _, item := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func(it models.PluginItem) {
			defer wg.Done()
			defer func() { <-sem }()

			short := strings.TrimPrefix(it.ID, "luci-app-")
			srcDesc := strings.TrimSpace(it.Desc)
			if srcDesc == "" || srcDesc == it.ID || srcDesc == short || strings.Contains(srcDesc, "扩展插件") {
				srcDesc = ""
			}

			var entry PluginI18nEntry
			isTrans := false
			if onlineOK && srcDesc != "" {
				if zhDesc := translateEnToZh(client, srcDesc); zhDesc != "" {
					zhName := translateEnToZh(client, strings.ReplaceAll(short, "-", " "))
					if zhName == "" || strings.EqualFold(zhName, short) {
						zhName = short + " 管理"
					} else if !strings.Contains(strings.ToLower(zhName), strings.ToLower(short)) {
						zhName = zhName + " (" + short + ")"
					}
					entry = PluginI18nEntry{Name: zhName, Desc: zhDesc}
					isTrans = true
				}
			}
			if !isTrans {
				entry = smartKeywordZh(it.ID, it.Feed)
			}

			mu.Lock()
			userDict[it.ID] = entry
			if isTrans {
				translatedCount++
			} else {
				fallbackCount++
			}
			mu.Unlock()
		}(item)
	}
	wg.Wait()

	_ = saveUserPluginI18n(userDict)
	enriched := EnrichPlugins(items)
	mode := "在线并发翻译"
	if !onlineOK {
		mode = "在线翻译超时，已秒切本地智能归类"
	}
	msg := fmt.Sprintf("中文补全完成（%s）：翻译 %d 个，智能归类 %d 个（已永久缓存）", mode, translatedCount, fallbackCount)
	return enriched, msg
}
