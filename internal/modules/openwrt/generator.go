package openwrt

import (
	"SysInitPro/internal/models"
	"fmt"
	"strings"
)

func applyMirror(rawURL, mirror string) string {
	m := strings.TrimSpace(mirror)
	if m == "" {
		return rawURL
	}
	if !strings.HasSuffix(m, "/") {
		m += "/"
	}
	if strings.HasPrefix(rawURL, "https://github.com/") {
		return m + rawURL
	}
	return rawURL
}

func ResolveRepoInfo(branch, mirror string) (string, string) {
	var url, br string
	switch branch {
	case "immortalwrt-23.05":
		url, br = "https://github.com/immortalwrt/immortalwrt.git", "openwrt-23.05"
	case "openwrt-24.10":
		url, br = "https://github.com/openwrt/openwrt.git", "openwrt-24.10"
	case "lede":
		url, br = "https://github.com/coolsnowwolf/lede.git", "master"
	default:
		url, br = "https://github.com/immortalwrt/immortalwrt.git", "openwrt-24.10"
	}
	return applyMirror(url, mirror), br
}

func targetConfigBlock(target string) string {
	switch target {
	case "armsr_armv8":
		return "CONFIG_TARGET_armsr=y\nCONFIG_TARGET_armsr_armv8=y\nCONFIG_TARGET_armsr_armv8_DEVICE_generic=y\n"
	case "rockchip_armv8":
		return "CONFIG_TARGET_rockchip=y\nCONFIG_TARGET_rockchip_armv8=y\nCONFIG_TARGET_rockchip_armv8_DEVICE_friendlyarm_nanopi-r4s=y\n"
	default:
		return "CONFIG_TARGET_x86=y\nCONFIG_TARGET_x86_64=y\nCONFIG_TARGET_x86_64_DEVICE_generic=y\n"
	}
}

func hasFeed(feeds []string, target string) bool {
	for _, f := range feeds {
		if strings.EqualFold(strings.TrimSpace(f), target) {
			return true
		}
	}
	return false
}

func buildBundleRaw(cfg models.FirmwareConfig) models.FirmwareBundle {
	if strings.TrimSpace(cfg.LanIP) == "" {
		cfg.LanIP = "192.168.100.1"
	}
	theme := strings.TrimSpace(cfg.Theme)
	if theme == "" || theme == "argon" || theme == "argone" {
		theme = "bootstrap"
	}

	var p1 strings.Builder
	p1.WriteString("#!/bin/bash\nset -e\n")
	if hasFeed(cfg.Feeds, "kenzok8") {
		p1.WriteString(fmt.Sprintf("sed -i '1i src-git kenzo %s' feeds.conf.default\n",
			applyMirror("https://github.com/kenzok8/openwrt-packages", cfg.GitMirror)))
		p1.WriteString(fmt.Sprintf("sed -i '2i src-git small %s' feeds.conf.default\n",
			applyMirror("https://github.com/kenzok8/small", cfg.GitMirror)))
	}
	if hasFeed(cfg.Feeds, "istore") {
		p1.WriteString(fmt.Sprintf("echo 'src-git istore %s;main' >> feeds.conf.default\n",
			applyMirror("https://github.com/linkease/istore", cfg.GitMirror)))
	}
	switch theme {
	case "kucat":
		p1.WriteString(fmt.Sprintf("git clone --depth 1 %s package/luci-theme-kucat\n",
			applyMirror("https://github.com/sirpdboy/luci-theme-kucat.git", cfg.GitMirror)))
	case "design":
		p1.WriteString(fmt.Sprintf("git clone --depth 1 %s package/luci-theme-design.git\n",
			applyMirror("https://github.com/0x676e67/luci-theme-design.git", cfg.GitMirror)))
	}
	p1.WriteString("./scripts/feeds update -a\n./scripts/feeds install -a -f\n")

	var p2 strings.Builder
	p2.WriteString("#!/bin/bash\nset -e\n")
	p2.WriteString(fmt.Sprintf("sed -i 's/192.168.1.1/%s/g' package/base-files/files/bin/config_generate\n", cfg.LanIP))
	p2.WriteString(fmt.Sprintf("sed -i 's/luci-theme-bootstrap/luci-theme-%s/g' feeds/luci/collections/luci/Makefile || true\n", theme))
	if strings.TrimSpace(cfg.RootPassword) != "" {
		p2.WriteString(fmt.Sprintf("echo 'root:%s' | chpasswd || true\n", cfg.RootPassword))
	}

	var dc strings.Builder
	dc.WriteString(targetConfigBlock(cfg.Target))
	dc.WriteString(fmt.Sprintf("CONFIG_PACKAGE_luci-theme-%s=y\n", theme))
	for _, pkg := range cfg.Plugins {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			dc.WriteString(fmt.Sprintf("CONFIG_PACKAGE_%s=y\n", pkg))
		}
	}

	repoURL, repoBranch := ResolveRepoInfo(cfg.Branch, cfg.GitMirror)
	wf := fmt.Sprintf(`name: Build OpenWrt Firmware (SysInitPro v10)
on:
  workflow_dispatch:
  push:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4
      - name: Install Dependencies
        run: |
          sudo apt-get update
          sudo apt-get install -y build-essential clang flex bison g++ gawk gcc-multilib g++-multilib gettext git libncurses-dev libssl-dev python3-distutils rsync unzip zlib1g-dev file wget
      - name: Clone & Build OpenWrt
        run: |
          git clone --depth 1 -b %s %s openwrt
          cd openwrt
          cp ../diy-part1.sh . && chmod +x diy-part1.sh && ./diy-part1.sh
          cp ../diy-part2.sh ../diff.config . && chmod +x diy-part2.sh && ./diy-part2.sh
          cp diff.config .config
          make defconfig
          make download -j$(nproc)
          make -j$(nproc) || make -j1 V=s
      - uses: actions/upload-artifact@v4
        with:
          name: OpenWrt-Firmware-%s
          path: openwrt/bin/targets/*/*/*.img.gz
`, repoBranch, repoURL, cfg.Target)

	return models.FirmwareBundle{
		DiyPart1:    p1.String(),
		DiyPart2:    p2.String(),
		DotConfig:   dc.String(),
		WorkflowYml: wf,
	}
}
