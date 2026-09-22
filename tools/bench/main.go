// 轻量 HTTP 压测工具：并发打一个接口，输出 QPS 和分位延迟
// 用法:
//   go run ./tools/bench -url http://localhost:8081/api/v1/video/feed -c 50 -d 15s
//   go run ./tools/bench -url ... -c 20 -d 10s -token <jwt> -method POST -body '{"video_id":1,"action_type":1}'
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var (
		url    string
		method string
		token  string
		body   string
		conc   int
		dur    time.Duration
	)
	flag.StringVar(&url, "url", "", "目标 URL")
	flag.StringVar(&method, "method", "GET", "HTTP 方法")
	flag.StringVar(&token, "token", "", "Bearer token (可选)")
	flag.StringVar(&body, "body", "", "请求体 (POST 用, 可选)")
	flag.IntVar(&conc, "c", 10, "并发数")
	flag.DurationVar(&dur, "d", 10*time.Second, "持续时间")
	flag.Parse()

	if url == "" {
		fmt.Println("缺少 -url")
		return
	}

	client := &http.Client{Timeout: 60 * time.Second}
	doOnce := func() (time.Duration, bool) {
		req, err := http.NewRequest(method, url, strings.NewReader(body))
		if err != nil {
			return 0, false
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		start := time.Now()
		resp, err := client.Do(req)
		lat := time.Since(start)
		if err != nil {
			return lat, false
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return lat, resp.StatusCode == 200
	}

	// 预热：建立连接池、触发 Redis/内核缓存
	for i := 0; i < 5; i++ {
		doOnce()
	}

	var total, okCount int64
	latencies := make([]time.Duration, 0, 100000)
	var mu sync.Mutex
	deadline := time.Now().Add(dur)
	var wg sync.WaitGroup

	wallStart := time.Now()
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]time.Duration, 0, 1024)
			for time.Now().Before(deadline) {
				lat, ok := doOnce()
				atomic.AddInt64(&total, 1)
				if ok {
					atomic.AddInt64(&okCount, 1)
				}
				local = append(local, lat)
			}
			mu.Lock()
			latencies = append(latencies, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	wall := time.Since(wallStart)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	pct := func(p float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		idx := int(float64(len(latencies)-1) * p)
		return latencies[idx]
	}

	fmt.Printf("目标:   %s %s\n", method, url)
	fmt.Printf("并发:   %d  持续: %s\n", conc, dur)
	fmt.Printf("请求数: %d  成功: %d (%.1f%%)\n", total, okCount, float64(okCount)/float64(total)*100)
	fmt.Printf("QPS:    %.1f\n", float64(total)/wall.Seconds())
	fmt.Printf("延迟:   avg=%s P50=%s P90=%s P99=%s max=%s\n",
		time.Duration(int64(avg(latencies))), pct(0.50), pct(0.90), pct(0.99), latencies[len(latencies)-1])
}

func avg(l []time.Duration) float64 {
	if len(l) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range l {
		sum += d
	}
	return float64(sum) / float64(len(l))
}
