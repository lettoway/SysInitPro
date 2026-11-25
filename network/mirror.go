package network

import (
	"net/http"
	"time"
)

type MirrorNode struct {
	Name    string        `json:"name"`
	BaseURL string        `json:"base_url"`
	Latency time.Duration `json:"latency"`
}

func GetAllMirrors() []MirrorNode {
	return []MirrorNode{
		{Name: "🌐 官方默认源 (Official / 直连上游)", BaseURL: "official"},
		{Name: "清华大学开源镜像站 (TUNA)", BaseURL: "https://mirrors.tuna.tsinghua.edu.cn"},
		{Name: "阿里云开源镜像站 (Aliyun)", BaseURL: "https://mirrors.aliyun.com"},
		{Name: "中国科学技术大学镜像站 (USTC)", BaseURL: "https://mirrors.ustc.edu.cn"},
	}
}

func BenchmarkMirrors() MirrorNode {
	mirrors := []MirrorNode{
		{Name: "清华大学开源镜像站", BaseURL: "https://mirrors.tuna.tsinghua.edu.cn"},
		{Name: "阿里云开源镜像站", BaseURL: "https://mirrors.aliyun.com"},
		{Name: "中国科学技术大学镜像站", BaseURL: "https://mirrors.ustc.edu.cn"},
	}
	client := http.Client{Timeout: 3 * time.Second}
	fastest := mirrors[0]
	fastest.Latency = 999 * time.Second
	for i := range mirrors {
		start := time.Now()
		resp, err := client.Head(mirrors[i].BaseURL)
		if err == nil {
			resp.Body.Close()
			mirrors[i].Latency = time.Since(start)
			if mirrors[i].Latency < fastest.Latency {
				fastest = mirrors[i]
			}
		}
	}
	return fastest
}