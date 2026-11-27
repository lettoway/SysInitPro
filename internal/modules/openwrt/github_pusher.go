package openwrt

import (
	"SysInitPro/internal/models"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func putGitHubFile(client *http.Client, repo, token, path, content, message string) error {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, path)

	// 先查文件是否已存在以获取 sha
	var sha string
	getReq, _ := http.NewRequest("GET", apiURL, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getReq.Header.Set("User-Agent", "SysInitPro-v10")
	if resp, err := client.Do(getReq); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			var info struct {
				SHA string `json:"sha"`
			}
			if json.NewDecoder(resp.Body).Decode(&info) == nil {
				sha = info.SHA
			}
		}
	}

	payload := map[string]string{
		"message": message,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
	}
	if sha != "" {
		payload["sha"] = sha
	}
	body, _ := json.Marshal(payload)

	putReq, err := http.NewRequest("PUT", apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("User-Agent", "SysInitPro-v10")
	putReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(putReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("推送 %s 失败 (HTTP %d)", path, resp.StatusCode)
	}
	return nil
}

func PushToGitHub(repo, token string, cfg models.FirmwareConfig) error {
	repo = strings.TrimSpace(repo)
	token = strings.TrimSpace(token)
	if !strings.Contains(repo, "/") || token == "" {
		return fmt.Errorf("请填写完整的仓库名（格式: 用户名/仓库名）与 GitHub Token")
	}
	bundle := BuildBundle(cfg)
	client := &http.Client{Timeout: 20 * time.Second}

	files := []struct {
		path    string
		content string
	}{
		{"diy-part1.sh", bundle.DiyPart1},
		{"diy-part2.sh", bundle.DiyPart2},
		{"diff.config", bundle.DotConfig},
		{".github/workflows/build-openwrt.yml", bundle.WorkflowYml},
	}

	for _, f := range files {
		if err := putGitHubFile(client, repo, token, f.path, f.content, "SysInitPro v10: update "+f.path); err != nil {
			return err
		}
	}
	return nil
}
