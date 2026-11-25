package openwrt

import (
	"fmt"
	"strings"
)

type FirmwareConfig struct {
	Branch       string   `json:"branch"`
	TargetArch   string   `json:"target_arch"`
	FeedSource   string   `json:"feed_source"`
	ThirdParty   []string `json:"third_party"`
	CustomFeed   string   `json:"custom_feed"`
	LanIP        string   `json:"lan_ip"`
	Hostname     string   `json:"hostname"`
	WifiSSID     string   `json:"wifi_ssid"`
	WifiPassword string   `json:"wifi_password"`
	DefaultTheme string   `json:"default_theme"`
	RootfsSizeMB int      `json:"rootfs_size_mb"`
	SelectedPkgs []string `json:"selected_pkgs"`
}

type GeneratedBundle struct {
	DiyPart1     string `json:"diy_part1"`
	DiyPart2     string `json:"diy_part2"`
	DotConfig    string `json:"dot_config"`
	WorkflowYml  string `json:"workflow_yml"`
	DockerScript string `json:"docker_script"`
}

func BuildFirmwareBundle(cfg FirmwareConfig) GeneratedBundle {
	return GeneratedBundle{
		DiyPart1:     generateDiyPart1(cfg),
		DiyPart2:     generateDiyPart2(cfg),
		DotConfig:    generateDotConfig(cfg),
		WorkflowYml:  generateWorkflowYml(cfg),
		DockerScript: generateDockerScript(cfg),
	}
}

func resolveRepoInfo(branch string) (string, string) {
	switch branch {
	case "openwrt":
		return "https://github.com/openwrt/openwrt", "v23.05.5"
	case "lede":
		return "https://github.com/coolsnowwolf/lede", "master"
	default:
		return "https://github.com/immortalwrt/immortalwrt", "openwrt-23.05"
	}
}

func generateDiyPart1(cfg FirmwareConfig) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	hasExtra := false
	for _, tp := range cfg.ThirdParty {
		if tp == "kenzok8" {
			hasExtra = true
			sb.WriteString("echo 'src-git kenzok8 https://github.com/kenzok8/openwrt-packages' >> feeds.conf.default\necho 'src-git small https://github.com/kenzok8/small' >> feeds.conf.default\n")
		} else if tp == "istore" {
			hasExtra = true
			sb.WriteString("echo 'src-git istore https://github.com/linkease/istore;main' >> feeds.conf.default\n")
		}
	}
	if strings.TrimSpace(cfg.CustomFeed) != "" {
		hasExtra = true
		sb.WriteString(fmt.Sprintf("echo '%s' >> feeds.conf.default\n", strings.TrimSpace(cfg.CustomFeed)))
	}
	if hasExtra {
		sb.WriteString("./scripts/feeds update -a\nrm -rf feeds/luci/applications/luci-app-passwall feeds/luci/applications/luci-app-openclash feeds/packages/net/xray-core feeds/packages/net/sing-box\n./scripts/feeds install -a -f\n")
	} else {
		sb.WriteString("./scripts/feeds update -a && ./scripts/feeds install -a\n")
	}
	return sb.String()
}

func generateDiyPart2(cfg FirmwareConfig) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	if cfg.LanIP != "" && cfg.LanIP != "192.168.1.1" {
		sb.WriteString(fmt.Sprintf("sed -i 's/192.168.1.1/%s/g' package/base-files/files/bin/config_generate\n", cfg.LanIP))
	}
	if cfg.Hostname != "" {
		sb.WriteString(fmt.Sprintf("sed -i 's/OpenWrt/%s/g' package/base-files/files/bin/config_generate\nsed -i 's/ImmortalWrt/%s/g' package/base-files/files/bin/config_generate\n", cfg.Hostname, cfg.Hostname))
	}
	if cfg.DefaultTheme != "" {
		sb.WriteString(fmt.Sprintf("sed -i 's/luci-theme-bootstrap/%s/g' feeds/luci/collections/luci/Makefile 2>/dev/null || true\n", cfg.DefaultTheme))
	}
	return sb.String()
}

func generateDotConfig(cfg FirmwareConfig) string {
	var sb strings.Builder
	switch cfg.TargetArch {
	case "mediatek_filogic":
		sb.WriteString("CONFIG_TARGET_mediatek=y\nCONFIG_TARGET_mediatek_filogic=y\nCONFIG_TARGET_MULTI_PROFILE=y\n")
	case "rockchip_armv8":
		sb.WriteString("CONFIG_TARGET_rockchip=y\nCONFIG_TARGET_rockchip_armv8=y\nCONFIG_TARGET_MULTI_PROFILE=y\n")
	default:
		sb.WriteString("CONFIG_TARGET_x86=y\nCONFIG_TARGET_x86_64=y\nCONFIG_TARGET_x86_64_DEVICE_generic=y\n")
	}
	if cfg.RootfsSizeMB > 0 {
		sb.WriteString(fmt.Sprintf("CONFIG_TARGET_ROOTFS_PARTSIZE=%d\n", cfg.RootfsSizeMB))
	}
	if cfg.DefaultTheme != "" {
		sb.WriteString(fmt.Sprintf("CONFIG_PACKAGE_%s=y\n", cfg.DefaultTheme))
	}
	for _, pkg := range cfg.SelectedPkgs {
		sb.WriteString(fmt.Sprintf("CONFIG_PACKAGE_%s=y\n", pkg))
	}
	return sb.String()
}

func generateDockerScript(cfg FirmwareConfig) string {
	repoURL, repoBranch := resolveRepoInfo(cfg.Branch)
	return fmt.Sprintf("#!/bin/bash\nmkdir -p firmware_output\ndocker run --rm -it -v \"$(pwd)/diy-part1.sh:/work/diy-part1.sh\" -v \"$(pwd)/diy-part2.sh:/work/diy-part2.sh\" -v \"$(pwd)/diff.config:/work/diff.config\" -v \"$(pwd)/firmware_output:/work/output\" ubuntu:22.04 bash -c \"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y build-essential clang flex bison g++ gawk gcc-multilib g++-multilib gettext git libncurses-dev libssl-dev python3-distutils rsync unzip zlib1g-dev file wget && useradd -m builder && su - builder -c 'git clone --depth 1 -b %s %s /home/builder/openwrt && cd /home/builder/openwrt && cp /work/diy-part1.sh . && chmod +x diy-part1.sh && ./diy-part1.sh && cp /work/diy-part2.sh /work/diff.config . && chmod +x diy-part2.sh && ./diy-part2.sh && cp diff.config .config && make defconfig && make download -j\\$(nproc) && (make -j\\$(nproc) || make -j1 V=s) && find bin/targets/ -type f \\( -name \\\"*.img.gz\\\" -o -name \\\"*.bin\\\" \\) -exec cp {} /work/output/ \\;'\"\n", repoBranch, repoURL)
}

func generateWorkflowYml(cfg FirmwareConfig) string {
	repoURL, repoBranch := resolveRepoInfo(cfg.Branch)
	return fmt.Sprintf("name: Build OpenWrt\non:\n  workflow_dispatch:\npermissions:\n  contents: write\njobs:\n  build:\n    runs-on: ubuntu-22.04\n    steps:\n      - uses: actions/checkout@v4\n      - run: sudo apt-get update && sudo apt-get install -y build-essential clang flex bison g++ gawk gcc-multilib g++-multilib gettext git libncurses-dev libssl-dev python3-distutils rsync unzip zlib1g-dev file wget\n      - run: git clone --depth 1 -b %s %s openwrt\n      - run: cp diy-part1.sh diy-part2.sh diff.config openwrt/ && cd openwrt && chmod +x diy-part1.sh diy-part2.sh && ./diy-part1.sh && ./diy-part2.sh && cp diff.config .config && make defconfig && make download -j$(nproc) && (make -j$(nproc) || make -j1 V=s) && mkdir -p ../artifacts && find bin/targets/ -type f \\( -name '*.img.gz' -o -name '*.bin' \\) -exec cp {} ../artifacts/ \\;\n      - uses: actions/upload-artifact@v4\n        with:\n          name: OpenWrt-Firmware\n          path: artifacts/*\n", repoBranch, repoURL)
}