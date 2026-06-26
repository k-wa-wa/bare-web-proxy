package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type TestPattern struct {
	Name string
	Path string
}

func main() {
	patterns := []TestPattern{
		{"Simple HTML", "/simple"},
		{"Standard CSS", "/css-standard"},
		{"Massive CSS (100KB)", "/css-massive"},
		{"Dynamic SPA (3s delay)", "/spa"},
		{"Slow Loading CSS (2s)", "/slow-loading"},
	}

	proxyBase := "http://localhost:3000/proxy"
	mockLocalBase := "http://localhost:3003"
	mockInternalBase := "http://bare-web-proxy-mock-service:3003" // K8s内のChromeサイドカーから見たアドレス

	iterations := 5

	fmt.Println("=== Bare Web Proxy 性能測定ベンチマーク ===")
	fmt.Printf("測定回数: %d回平均 | プロキシ: %s | モックサーバー: %s\n", iterations, proxyBase, mockLocalBase)
	fmt.Println("------------------------------------------------------------------------------------------------------")
	fmt.Printf("| %-20s | %-14s | %-14s | %-10s | %-12s | %-12s | %-10s |\n",
		"テストパターン", "直接サイズ", "プロキシサイズ", "削減率", "直接Latency", "プロキシLat.", "レイテンシ差")
	fmt.Println("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |")

	for _, p := range patterns {
		// 1. 直接アクセスのトータルサイズとレイテンシを測定
		directSize, err := getDirectTotalSize(mockLocalBase + p.Path)
		if err != nil {
			log.Printf("[ERROR] 直接サイズ取得失敗 (%s): %v", p.Name, err)
			continue
		}

		directLatency, err := measureLatency(mockLocalBase+p.Path, iterations)
		if err != nil {
			log.Printf("[ERROR] 直接レイテンシ測定失敗 (%s): %v", p.Name, err)
			continue
		}

		// 2. プロキシ経由のサイズとレイテンシを測定
		proxyTarget := fmt.Sprintf("%s?url=%s", proxyBase, url.QueryEscape(mockInternalBase+p.Path))
		proxySize, err := getURLContentSize(proxyTarget)
		if err != nil {
			log.Printf("[ERROR] プロキシサイズ取得失敗 (%s): %v", p.Name, err)
			continue
		}

		proxyLatency, err := measureLatency(proxyTarget, iterations)
		if err != nil {
			log.Printf("[ERROR] プロキシレイテンシ測定失敗 (%s): %v", p.Name, err)
			continue
		}

		// 圧縮率とレイテンシ差の計算
		reduction := 0.0
		if directSize > 0 {
			reduction = float64(directSize-proxySize) / float64(directSize) * 100
		}
		latencyDiff := proxyLatency - directLatency

		fmt.Printf("| %-20s | %9.2f KB     | %9.2f KB     | %8.1f%% | %8d ms    | %8d ms    | %+8d ms    |\n",
			p.Name,
			float64(directSize)/1024.0,
			float64(proxySize)/1024.0,
			reduction,
			directLatency.Milliseconds(),
			proxyLatency.Milliseconds(),
			latencyDiff.Milliseconds(),
		)
	}
	fmt.Println("------------------------------------------------------------------------------------------------------")
}

// getDirectTotalSize はHTMLとその中に含まれる外部CSSのトータルサイズを計算する
func getDirectTotalSize(targetURL string) (int, error) {
	resp, err := http.Get(targetURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	htmlSize := len(bodyBytes)

	// HTMLをパースして外部CSSを探す
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return htmlSize, nil // パース失敗時はHTMLのみのサイズを返す
	}

	parsedURL, _ := url.Parse(targetURL)
	cssTotalSize := 0

	doc.Find("link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// 相対パスを解決
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		resolvedURL := parsedURL.ResolveReference(u).String()

		// CSSの中身を取得してサイズを加算
		cssSize, err := getURLContentSize(resolvedURL)
		if err == nil {
			cssTotalSize += cssSize
		}
	})

	return htmlSize + cssTotalSize, nil
}

// getURLContentSize は指定したURLのレスポンスサイズを取得する
func getURLContentSize(targetURL string) (int, error) {
	resp, err := http.Get(targetURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	return len(bodyBytes), nil
}

// measureLatency は指定したURLのレスポンスタイムの平均を測定する
func measureLatency(targetURL string, iterations int) (time.Duration, error) {
	var total time.Duration
	successCount := 0

	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, err := http.Get(targetURL)
		if err != nil {
			continue
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			total += time.Since(start)
			successCount++
		}
		time.Sleep(50 * time.Millisecond) // モックサーバーへの負荷軽減
	}

	if successCount == 0 {
		return 0, fmt.Errorf("all requests failed for %s", targetURL)
	}

	return total / time.Duration(successCount), nil
}
