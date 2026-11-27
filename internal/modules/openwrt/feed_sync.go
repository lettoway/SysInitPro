package openwrt

import (
	v10assets "SysInitPro/internal/assets"
	"SysInitPro/internal/models"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ghTreeResp struct {
	Truncated bool `json:"truncated"`
	Tree      []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
}

type ghContentItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ghRateResp struct {
	Resources struct {
		Core struct {
			Limit     int `json:"limit"`
			Remaining int `json:"remaining"`
		} `json:"core"`
	} `json:"resources"`
}

var ghTreeAppRe = regexp.MustCompile(`"name":"(luci-app-[a-zA-Z0-9._-]+)"`)

func feedsCachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "SysInitPro")
	_ = os.MkdirAll(appDir, 0755)
	return filepath.Join(appDir, "feeds_cache.json"), nil
}

func LoadCachedPlugins() []models.PluginItem {
	p, err := feedsCachePath()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var items []models.PluginItem
	if json.Unmarshal(b, &items) != nil {
		return nil
	}
	return items
}

// VerifyGitHubToken 实测 Token 有效性并返回真实剩余额度
func VerifyGitHubToken(client *http.Client, token string) (string, int, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/rate_limit", nil)
	req.Header.Set("User-Agent", "SysInitPro-v10")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return "", 0, fmt.Errorf("Token 无效或已过期 (HTTP 401)")
	}
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("额度检测失败 (HTTP %d)", resp.StatusCode)
	}
	var rr ghRateResp
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return "", 0, err
	}
	mode := "匿名模式"
	if token != "" {
		mode = "Token 有效"
	}
	return fmt.Sprintf("[%s] GitHub API 剩余额度: %d / %d", mode, rr.Resources.Core.Remaining, rr.Resources.Core.Limit), rr.Resources.Core.Remaining, nil
}

func fetchOfficialPackages(client *http.Client) (map[string]string, error) {
	urls := []string{
		"https://downloads.openwrt.org/releases/24.10.0/packages/x86_64/luci/Packages",
		"https://downloads.immortalwrt.org/releases/24.10.0/packages/x86_64/luci/Packages",
	}
	var lastErr error
	for _, u := range urls {
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", "SysInitPro-v10")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		res := make(map[string]string)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
		var curPkg string
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "Package: ") {
				pkg := strings.TrimSpace(strings.TrimPrefix(line, "Package: "))
				if strings.HasPrefix(pkg, "luci-app-") {
					curPkg = pkg
					if _, ok := res[curPkg]; !ok {
						res[curPkg] = curPkg
					}
				} else {
					curPkg = ""
				}
			} else if curPkg != "" && strings.HasPrefix(line, "Description: ") {
				desc := strings.TrimSpace(strings.TrimPrefix(line, "Description: "))
				if desc != "" {
					res[curPkg] = desc
				}
			}
		}
		resp.Body.Close()
		if len(res) > 0 {
			return res, nil
		}
	}
	return nil, fmt.Errorf("官方索引拉取失败: %v", lastErr)
}

// 非递归读取仓库根目录树（不会因嵌套源码文件过多被 GitHub 截断）
func fetchGitHubRootTree(client *http.Client, repo, branch, token string) ([]string, bool, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s", repo, branch)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "SysInitPro-v10")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var tr ghTreeResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, false, err
	}
	var names []string
	for _, node := range tr.Tree {
		if node.Type == "tree" && strings.HasPrefix(node.Path, "luci-app-") {
			names = append(names, node.Path)
		}
	}
	return names, tr.Truncated, nil
}

// 精确读取指定子目录内容
func fetchGitHubSubdir(client *http.Client, repo, subpath, token string) ([]string, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, strings.TrimPrefix(subpath, "/"))
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "SysInitPro-v10")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var entries []ghContentItem
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type == "dir" && strings.HasPrefix(e.Name, "luci-app-") {
			names = append(names, e.Name)
		}
	}
	return names, nil
}

// 降级通道：网页 HTML 解析（仅在 API 限流时兜底）
func fetchViaGitHubWeb(client *http.Client, repo, branch, subpath string) ([]string, error) {
	u := fmt.Sprintf("https://github.com/%s/tree/%s", repo, branch)
	if subpath != "" {
		u += "/" + strings.TrimPrefix(subpath, "/")
	}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Web HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 6*1024*1024))
	if err != nil {
		return nil, err
	}
	matches := ghTreeAppRe.FindAllStringSubmatch(string(body), -1)
	seen := make(map[string]struct{})
	var names []string
	for _, m := range matches {
		if len(m) > 1 {
			if _, ok := seen[m[1]]; !ok {
				seen[m[1]] = struct{}{}
				names = append(names, m[1])
			}
		}
	}
	return names, nil
}

func SyncAllFeeds(token string) ([]models.PluginItem, string, error) {
	baseCat, _ := v10assets.LoadOpenWrtCatalog()
	knownInfo := make(map[string]models.PluginItem)
	newByFeed := map[string]map[string]models.PluginItem{
		"official": {},
		"kenzok8":  {},
		"istore":   {},
	}
	for _, p := range baseCat.Plugins {
		knownInfo[p.ID] = p
		if newByFeed[p.Feed] == nil {
			newByFeed[p.Feed] = map[string]models.PluginItem{}
		}
		newByFeed[p.Feed][p.ID] = p
	}

	addPlugin := func(feed, id, fallbackDesc string) {
		if _, exists := newByFeed[feed][id]; exists {
			return
		}
		name := strings.TrimPrefix(id, "luci-app-")
		desc := fallbackDesc
		defOn := false
		if k, ok := knownInfo[id]; ok {
			name = k.Name
			desc = k.Desc
			defOn = k.DefaultOn
		}
		newByFeed[feed][id] = models.PluginItem{
			ID:        id,
			Name:      name,
			Desc:      desc,
			Feed:      feed,
			DefaultOn: defOn,
		}
	}

	client := &http.Client{Timeout: 25 * time.Second}
	token = strings.TrimSpace(token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}

	var details []string
	needToken := false

	// 0. 实测 GitHub Token 与当前剩余额度
	rateMsg, remaining, err := VerifyGitHubToken(client, token)
	if err != nil {
		return nil, "", err
	}
	details = append(details, rateMsg)
	if remaining < 5 {
		needToken = true
	}

	// 1. 官方源
	if offMap, err := fetchOfficialPackages(client); err != nil {
		details = append(details, fmt.Sprintf("• official: 失败 (%v)", err))
	} else {
		for id, desc := range offMap {
			addPlugin("official", id, desc)
		}
		details = append(details, fmt.Sprintf("• official: %d 个（官方 Packages 索引，精确完整）", len(newByFeed["official"])))
	}

	// 2. 第三方源（优先走精确 GitHub API，无额度时降级网页并提示）
	sources := []struct {
		feed    string
		repo    string
		branch  string
		subpath string
	}{
		{"kenzok8", "kenzok8/openwrt-packages", "master", ""},
		{"istore", "linkease/istore", "main", "luci"},
		{"istore", "linkease/nas-packages-luci", "main", "luci"},
	}

	for _, src := range sources {
		var apps []string
		var exact bool
		var apiErr error

		if remaining > 0 {
			if src.subpath == "" {
				var trunc bool
				apps, trunc, apiErr = fetchGitHubRootTree(client, src.repo, src.branch, token)
				exact = (apiErr == nil && !trunc)
			} else {
				apps, apiErr = fetchGitHubSubdir(client, src.repo, src.subpath, token)
				exact = (apiErr == nil)
			}
			if apiErr == nil {
				remaining--
			}
		} else {
			apiErr = fmt.Errorf("HTTP 403 (API 额度已耗尽)")
		}

		if apiErr != nil {
			if strings.Contains(apiErr.Error(), "403") || strings.Contains(apiErr.Error(), "429") {
				needToken = true
			}
			webApps, wErr := fetchViaGitHubWeb(client, src.repo, src.branch, src.subpath)
			if wErr == nil && len(webApps) > 0 {
				apps = webApps
				details = append(details, fmt.Sprintf("• %s (%s): %d 个（⚠️ 降级网页解析，API 原因: %v）", src.feed, src.repo, len(apps), apiErr))
			} else {
				details = append(details, fmt.Sprintf("• %s (%s): 失败 (%v)", src.feed, src.repo, apiErr))
				continue
			}
		} else {
			tag := "精确完整"
			if !exact {
				tag = "⚠️ 仓库返回已截断"
			}
			details = append(details, fmt.Sprintf("• %s (%s): %d 个（GitHub API，%s）", src.feed, src.repo, len(apps), tag))
		}

		for _, id := range apps {
			addPlugin(src.feed, id, id)
		}
	}

	var result []models.PluginItem
	for _, feed := range []string{"official", "kenzok8", "istore"} {
		for _, item := range newByFeed[feed] {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DefaultOn != result[j].DefaultOn {
			return result[i].DefaultOn
		}
		if result[i].Feed != result[j].Feed {
			return result[i].Feed < result[j].Feed
		}
		return result[i].ID < result[j].ID
	})

	if p, err := feedsCachePath(); err == nil {
		if data, err := json.MarshalIndent(result, "", "  "); err == nil {
			_ = os.WriteFile(p, data, 0644)
		}
	}

	header := fmt.Sprintf("同步完成：共收录 %d 条（官方: %d | KenzoK8: %d | iStore: %d）",
		len(result), len(newByFeed["official"]), len(newByFeed["kenzok8"]), len(newByFeed["istore"]))
	if needToken {
		header += " [触发限流]"
	}
	return result, header + "\n" + strings.Join(details, "\n"), nil
}
