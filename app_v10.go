package main

import (
	v10assets "SysInitPro/internal/assets"
	"SysInitPro/internal/engine/store"
	"SysInitPro/internal/engine/task"
	"SysInitPro/internal/models"
	owrtv10 "SysInitPro/internal/modules/openwrt"
	pvev10 "SysInitPro/internal/modules/pve"
	"context"
	"os"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var v10TaskMgr = task.NewManager()

func (a *App) V10GetOpenWrtCatalog() (models.OpenWrtCatalog, error) {
	cat, err := v10assets.LoadOpenWrtCatalog()
	if err != nil {
		return cat, err
	}
	if cached := owrtv10.LoadCachedPlugins(); len(cached) > 0 {
		cat.Plugins = owrtv10.EnrichPlugins(cached)
	}
	cat.Plugins = owrtv10.EnrichPlugins(cat.Plugins)
	return cat, nil
}

func (a *App) V10SyncOpenWrtFeeds(token string) map[string]interface{} {
	items, msg, err := owrtv10.SyncAllFeeds(token)
	if err != nil {
		rl := strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "429")
		return map[string]interface{}{"ok": false, "message": err.Error(), "rateLimited": rl}
	}
	rl := strings.Contains(msg, "[触发限流]") || strings.Contains(msg, "HTTP 403") || strings.Contains(msg, "HTTP 429")
	return map[string]interface{}{"ok": true, "message": msg, "plugins": owrtv10.EnrichPlugins(items), "rateLimited": rl}
}

func (a *App) V10PushOpenWrtGitHub(repo, token string, cfg models.FirmwareConfig) string {
	if err := owrtv10.PushToGitHub(repo, token, cfg); err != nil {
		return "[FAIL] " + err.Error()
	}
	return "[OK] 已推送至 GitHub 仓库 " + repo + "，请前往 Actions 页面查看云端编译进度"
}

func (a *App) V10LoadConfig() models.AppConfig {
	return store.Load()
}

func (a *App) V10SaveConfig(cfg models.AppConfig) error {
	return store.Save(cfg)
}

func (a *App) V10GetTaskStatus() models.TaskStatus {
	return v10TaskMgr.Status()
}

func (a *App) V10CancelTask() bool {
	return v10TaskMgr.Cancel()
}

func (a *App) V10PreviewOpenWrtBundle(cfg models.FirmwareConfig) models.FirmwareBundle {
	return owrtv10.BuildBundle(cfg)
}

func (a *App) V10StartOpenWrtBuild(cfg models.FirmwareConfig) string {
	err := v10TaskMgr.Run("openwrt-build", "OpenWrt Docker 编译", func(ctx context.Context) error {
		wd, _ := os.Getwd()
		return owrtv10.RunDockerBuild(ctx, wd, cfg, func(line string) {
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "v10:log", line)
			}
		})
	})
	if err != nil {
		return "[FAIL] " + err.Error()
	}
	return "[OK] OpenWrt 固件编译完成，请查看 firmware_output 目录"
}

func (a *App) V10ProbePVE(host, user, password string) pvev10.ProbeResult {
	return pvev10.ProbeHost(host, user, password)
}
func (a *App) V10CheckOpenWrtConflicts(cfg models.FirmwareConfig) []owrtv10.ConflictWarning {
	return owrtv10.CheckConflicts(cfg)
}

func (a *App) V10TranslateMissingPlugins() map[string]interface{} {
	cat, err := a.V10GetOpenWrtCatalog()
	if err != nil {
		return map[string]interface{}{"ok": false, "message": err.Error()}
	}
	items, msg := owrtv10.TranslateMissingPlugins(cat.Plugins)
	return map[string]interface{}{"ok": true, "message": msg, "plugins": items}
}
