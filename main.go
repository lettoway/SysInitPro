package main

import (
	"SysInitPro/config"
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	cliMode := flag.Bool("cli", false, "在无图形界面的服务器终端模式下运行")
	profilePath := flag.String("profile", "my-sysinit-profile.json", "CLI 模式下加载的 JSON 配置文件路径")
	flag.Parse()

	app := NewApp()
	if *cliMode {
		fmt.Println("⚡ SysInitPro v9.0 CLI 服务器静默初始化引擎启动...")
		prof, err := config.ImportProfile(*profilePath)
		if err != nil {
			fmt.Printf("❌ 加载配置文件失败 (%s): %v\n", *profilePath, err)
			os.Exit(1)
		}
		hw, _ := app.ScanHardware()
		app.ExecutePipelineSync(*prof, hw)
		return
	}

	err := wails.Run(&options.App{
		Title:            "SysInitPro v9.0 - 全栈装机、系统补丁、VPS 运维与 PVE 软路由工作台",
		Width:            1240,
		Height:           850,
		MinWidth:         1060,
		MinHeight:        720,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 11, G: 15, B: 25, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}