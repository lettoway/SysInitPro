package main

import (
	"SysInitPro/core"
	"fmt"
	"os"
)

func (a *App) CheckCodeIntegrity() string {
	wd, err := os.Getwd()
	if err != nil { wd = "." }
	return core.FormatIntegritySummary(core.VerifyProjectIntegrity(wd))
}

func (a *App) RefreshCodeManifest() string {
	wd, err := os.Getwd()
	if err != nil { wd = "." }
	count, err := core.RefreshIntegrityManifest(wd)
	if err != nil { return "[FAIL] 刷新完整性基准清单失败: " + err.Error() }
	return fmt.Sprintf("[OK] 已刷新 sysinit_manifest.sha256 基准清单（共记录 %d 个文件）", count)
}