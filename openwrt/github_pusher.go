package openwrt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GitHubPushRequest struct {
	Token    string         `json:"token"`
	Repo     string         `json:"repo"`
	Branch   string         `json:"branch"`
	FwConfig FirmwareConfig `json:"fw_config"`
}

func PushAndTriggerWorkflow(req GitHubPushRequest, logFn func(string)) (string, error) {
	token, repo, branch := strings.TrimSpace(req.Token), strings.TrimSpace(req.Repo), strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}
	if token == "" || !strings.Contains(repo, "/") {
		return "", fmt.Errorf("请填写有效的 GitHub Token 和仓库名")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	bundle := BuildFirmwareBundle(req.FwConfig)
	files := map[string]string{"diy-part1.sh": bundle.DiyPart1, "diy-part2.sh": bundle.DiyPart2, "diff.config": bundle.DotConfig, ".github/workflows/build-openwrt.yml": bundle.WorkflowYml}
	for path, content := range files {
		if logFn != nil {
			logFn("☁️ 正在推送: " + path)
		}
		if err := upsertGitHubFile(client, token, repo, branch, path, content); err != nil {
			return "", err
		}
	}
	time.Sleep(2 * time.Second)
	payload, _ := json.Marshal(map[string]string{"ref": branch})
	httpReq, _ := http.NewRequest("POST", fmt.Sprintf("https://api.github.com/repos/%s/actions/workflows/build-openwrt.yml/dispatches", repo), bytes.NewReader(payload))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("触发 Actions 失败 (%d): %s", resp.StatusCode, string(b))
	}
	return fmt.Sprintf("https://github.com/%s/actions", repo), nil
}

func upsertGitHubFile(client *http.Client, token, repo, branch, filePath, content string) error {
	var sha string
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s", repo, filePath, branch), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	if getResp, err := client.Do(getReq); err == nil {
		defer getResp.Body.Close()
		if getResp.StatusCode == 200 {
			var info struct{ SHA string `json:"sha"` }
			_ = json.NewDecoder(getResp.Body).Decode(&info)
			sha = info.SHA
		}
	}
	bodyMap := map[string]interface{}{"message": "chore(sysinit): update " + filePath, "content": base64.StdEncoding.EncodeToString([]byte(content)), "branch": branch}
	if sha != "" {
		bodyMap["sha"] = sha
	}
	b, _ := json.Marshal(bodyMap)
	putReq, _ := http.NewRequest("PUT", fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, filePath), bytes.NewReader(b))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putResp, err := client.Do(putReq)
	if err != nil {
		return err
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != 200 && putResp.StatusCode != 201 {
		rb, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("HTTP %d: %s", putResp.StatusCode, string(rb))
	}
	return nil
}