package xhttp

import (
	"log"
	"net/http"
	"time"
)

// Middleware 是HTTP中间件类型，用于包装 http.RoundTripper。
//
// 使用装饰器模式，每个中间件接收一个 http.RoundTripper，返回一个新的 http.RoundTripper。
// 多个中间件按注册顺序从外到内执行。
type Middleware func(next http.RoundTripper) http.RoundTripper

// roundTripFunc 将函数适配为 http.RoundTripper。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// LoggingMiddleware 创建一个简单的日志中间件。
// loggerFunc 接收格式化后的日志字符串。如果为 nil，默认使用 log.Printf。
func LoggingMiddleware(loggerFunc func(format string, v ...any)) Middleware {
	if loggerFunc == nil {
		loggerFunc = log.Printf
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.RoundTrip(req)
			duration := time.Since(start)

			status := 0
			if resp != nil {
				status = resp.StatusCode
			}

			if err != nil {
				loggerFunc("← %s %s | %d | %v | ERR: %v", req.Method, req.URL, status, duration, err)
			} else {
				loggerFunc("← %s %s | %d | %v", req.Method, req.URL, status, duration)
			}
			return resp, err
		})
	}
}

// ---------- 重试配置 ----------

// retryConfig 重试策略配置
type retryConfig struct {
	maxAttempts  int
	baseInterval time.Duration
	shouldRetry  func(statusCode int, err error) bool
}

// retryMiddleware 创建重试中间件。
// 该中间件位于中间件链的最内层，紧邻实际 Transport。
// 对非幂等方法（除 GET/HEAD/OPTIONS/DELETE 外）不自动重试，除非 shouldRetry 明确返回 true。
func retryMiddleware(cfg *retryConfig, method string) Middleware {
	// 判断方法是否为幂等（GET、HEAD、OPTIONS、DELETE 视为幂等）
	isIdempotent := method == http.MethodGet || method == http.MethodHead ||
		method == http.MethodOptions || method == http.MethodDelete

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var lastErr error
			var lastResp *http.Response

			for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
				// 如果不是第一次尝试且有 GetBody 方法，重置 Body 以防被前一次尝试消耗
				if attempt > 1 && req.GetBody != nil {
					newBody, err := req.GetBody()
					if err != nil {
						return nil, err
					}
					req.Body = newBody
				}

				resp, err := next.RoundTrip(req)
				lastResp = resp
				lastErr = err

				// 成功：无错误且状态码非 5xx，直接返回
				if err == nil && resp != nil && resp.StatusCode < 500 {
					return resp, nil
				}

				// 判断是否应重试
				statusCode := 0
				if resp != nil {
					statusCode = resp.StatusCode
				}

				if attempt == cfg.maxAttempts {
					break // 最后一次尝试，不重试
				}

				shouldRetry := false
				if cfg.shouldRetry != nil {
					shouldRetry = cfg.shouldRetry(statusCode, err)
				} else {
					// 默认：5xx 或网络错误重试
					shouldRetry = (err != nil) || (statusCode >= 500)
				}

				// 非幂等方法需要 shouldRetry 明确允许才重试
				if shouldRetry && !isIdempotent {
					if cfg.shouldRetry == nil {
						shouldRetry = false // 没有自定义判断，不重试
					}
				}

				if !shouldRetry {
					return resp, err
				}

				// 指数退避：1x, 2x, 4x, 8x...
				backoff := cfg.baseInterval * (1 << (attempt - 1))
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				case <-time.After(backoff):
				}
			}

			// 所有重试耗尽
			return lastResp, lastErr
		})
	}
}
