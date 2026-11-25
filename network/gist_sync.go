package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GistSyncResult struct {
	GistID  string `json:"gist_id"`
	HtmlURL string `json:"html_url"`
}

func SyncToGist(token, gistID, profileJSON, catalogJSON string) (*GistSyncResult, error) {
	token, gistID = strings.TrimSpace(token), strings.TrimSpace(gistID)
	if token == "" {
		return nil, fmt.Errorf("请填写具有 gist 权限的 GitHub Token")
	}
	payload := map[string]interface{}{
		"description": "SysInitPro 跨平台装机配置与软件目录备份",
		"public":      false,
		"files": map[string]interface{}{
			"sysinit_profile.json": map[string]string{"content": profileJSON},
			"sysinit_catalog.json": map[string]string{"content": catalogJSON},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	method, apiURL := "POST", "https://api.github.com/gists"
	if gistID != "" {
		method, apiURL = "PATCH", "https://api.github.com/gists/"+gistID
	}
	req, _ := http.NewRequest(method, apiURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub Gist API 返回 %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		ID      string `json:"id"`
		HtmlURL string `json:"html_url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return &GistSyncResult{GistID: res.ID, HtmlURL: res.HtmlURL}, nil
}

func RestoreFromGist(token, gistID string) (string, string, error) {
	gistID = strings.TrimSpace(gistID)
	if gistID == "" {
		return "", "", fmt.Errorf("请填写要恢复的 Gist ID")
	}
	req, _ := http.NewRequest("GET", "https://api.github.com/gists/"+gistID, nil)
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("读取 Gist 失败 (HTTP %d): %s", resp.StatusCode, string(raw))
	}
	var data struct {
		Files map[string]struct{ Content string `json:"content"` } `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", err
	}
	return data.Files["sysinit_profile.json"].Content, data.Files["sysinit_catalog.json"].Content, nil
}