package middleware

import (
	nethttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// 限流中间件单测：突发桶耗尽后返回 429
func TestRateLimitLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", RateLimitLogin(), func(c *gin.Context) { c.Status(nethttp.StatusOK) })

	srv := httptest.NewServer(r)
	defer srv.Close()

	status := make([]int, 0, 8)
	for i := 0; i < 8; i++ {
		resp, err := nethttp.Post(srv.URL+"/login", "application/json", nil)
		if err != nil {
			t.Fatalf("请求失败: %v", err)
		}
		resp.Body.Close()
		status = append(status, resp.StatusCode)
	}

	ok, limited := 0, 0
	for _, s := range status {
		switch s {
		case 200:
			ok++
		case 429:
			limited++
		}
	}
	// 配置为 0.5 QPS + 突发 5：前 5 次放行，之后 429
	if ok != 5 {
		t.Errorf("应放行恰好 5 次突发，实际 %d 次 (状态码: %v)", ok, status)
	}
	if limited == 0 {
		t.Errorf("突发耗尽后应出现 429 (状态码: %v)", status)
	}
}

// 并发安全冒烟：多 goroutine 同时打限流器不应 panic/data race
func TestRateLimitConcurrentSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", RateLimitLogin(), func(c *gin.Context) { c.Status(nethttp.StatusOK) })

	srv := httptest.NewServer(r)
	defer srv.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				resp, err := nethttp.Post(srv.URL+"/login", "application/json", nil)
				if err == nil {
					resp.Body.Close()
				}
			}
		}()
	}
	wg.Wait() // 走到这里且无 panic/race 即通过
}
