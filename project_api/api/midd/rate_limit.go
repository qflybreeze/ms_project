package midd

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// GlobalRateLimit 全局限流中间件
// r: 每秒生成的令牌数（QPS上限）
// b: 令牌桶容量（允许的瞬时突发量）
func GlobalRateLimit(r rate.Limit, b int) gin.HandlerFunc {
	limiter := rate.NewLimiter(r, b)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			zap.L().Warn("global rate limit exceeded",
				zap.String("uri", c.Request.RequestURI),
				zap.String("ip", c.ClientIP()),
			)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": http.StatusTooManyRequests,
				"msg":  "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// 管理每个 IP 的独立限流器
type ipLimiterStore struct {
	limiters sync.Map
	r        rate.Limit
	b        int
}

func newIPLimiterStore(r rate.Limit, b int) *ipLimiterStore {
	return &ipLimiterStore{r: r, b: b}
}

func (s *ipLimiterStore) getLimiter(ip string) *rate.Limiter {
	if v, ok := s.limiters.Load(ip); ok {
		return v.(*rate.Limiter)
	}
	limiter := rate.NewLimiter(s.r, s.b)
	s.limiters.Store(ip, limiter)
	return limiter
}

// PerIPRateLimit 按 IP 限流中间件
// r: 每个 IP 每秒允许的请求数
// b: 每个 IP 允许的突发量
func PerIPRateLimit(r rate.Limit, b int) gin.HandlerFunc {
	store := newIPLimiterStore(r, b)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := store.getLimiter(ip)
		if !limiter.Allow() {
			zap.L().Warn("per-ip rate limit exceeded",
				zap.String("uri", c.Request.RequestURI),
				zap.String("ip", ip),
			)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": http.StatusTooManyRequests,
				"msg":  "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
