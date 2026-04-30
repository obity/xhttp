package xhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/obity/xhttp/util"
)

// defaultSuccessCodes 默认的成功HTTP状态码集合
var defaultSuccessCodes = map[int]struct{}{
	http.StatusOK:      {},
	http.StatusCreated: {},
}

// Client 是可复用的HTTP客户端，支持超时、Transport配置、自定义成功状态码，
// 以及中间件链和重试策略。
// 通过Builder模式提供链式配置调用。
type Client struct {
	headerSetter HeaderSetter
	httpClient   *http.Client
	successCodes map[int]struct{}
	middlewares  []Middleware     // 中间件链（按注册顺序从外到内）
	retryConfig  *retryConfig    // 重试配置（nil 表示不重试）
}

// NewClient 创建新的Client实例，使用默认的成功状态码（200和201）。
// httpClient使用默认的空Client，可通过Builder方法进一步配置。
func NewClient(setter HeaderSetter) *Client {
	return &Client{
		headerSetter: setter,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		successCodes: cloneMap(defaultSuccessCodes),
	}
}

// copy 创建当前 Client 的深拷贝。
// httpClient 被新建，Transport 被克隆（若为 *http.Transport），successCodes 被深拷贝。
// 返回的新实例与原实例互不影响。
func (c *Client) copy() *Client {
	cc := &Client{
		headerSetter: c.headerSetter,
		httpClient: &http.Client{
			Timeout:       c.httpClient.Timeout,
			Transport:     c.httpClient.Transport,
			CheckRedirect: c.httpClient.CheckRedirect,
			Jar:           c.httpClient.Jar,
		},
		successCodes: cloneMap(c.successCodes),
		middlewares:  append([]Middleware{}, c.middlewares...), // 拷贝切片
	}
	if c.retryConfig != nil {
		r := *c.retryConfig
		cc.retryConfig = &r
	}
	// 克隆 Transport 以避免 Builder 方法修改共享 Transport 时影响原实例
	if t, ok := c.httpClient.Transport.(*http.Transport); ok {
		cc.httpClient.Transport = t.Clone()
	}
	return cc
}

// WithTimeout 设置HTTP客户端的超时时间。
// 超时包括连接、重定向和读取响应的整个过程。
// 返回新实例，原实例不受影响。
func (c *Client) WithTimeout(d time.Duration) *Client {
	cc := c.copy()
	cc.httpClient.Timeout = d
	return cc
}

// WithTransport 设置自定义的 http.RoundTripper。
// 可用于自定义证书、代理等高级传输层配置。
// 注意：设置自定义Transport后，WithMaxIdleConns/WithMaxIdleConnsPerHost/WithIdleConnTimeout
// 方法仅在Transport为 *http.Transport 类型时生效。
// 返回新实例，原实例不受影响。
func (c *Client) WithTransport(transport http.RoundTripper) *Client {
	cc := c.copy()
	cc.httpClient.Transport = transport
	return cc
}

// WithMaxIdleConns 设置连接池中最大空闲连接数。
// 若尚未设置Transport，会懒初始化一个基于 http.DefaultTransport 克隆的 Transport。
// 返回新实例，原实例不受影响。
func (c *Client) WithMaxIdleConns(n int) *Client {
	cc := c.copy()
	if t := cc.transport(); t != nil {
		t.MaxIdleConns = n
	}
	return cc
}

// WithMaxIdleConnsPerHost 设置每个主机最大空闲连接数。
// 若尚未设置Transport，会懒初始化一个基于 http.DefaultTransport 克隆的 Transport。
// 返回新实例，原实例不受影响。
func (c *Client) WithMaxIdleConnsPerHost(n int) *Client {
	cc := c.copy()
	if t := cc.transport(); t != nil {
		t.MaxIdleConnsPerHost = n
	}
	return cc
}

// WithIdleConnTimeout 设置空闲连接的超时时间。
// 若尚未设置Transport，会懒初始化一个基于 http.DefaultTransport 克隆的 Transport。
// 返回新实例，原实例不受影响。
func (c *Client) WithIdleConnTimeout(d time.Duration) *Client {
	cc := c.copy()
	if t := cc.transport(); t != nil {
		t.IdleConnTimeout = d
	}
	return cc
}

// WithSuccessCodes 替换Client级别的成功状态码集合。
// 不影响 defaultSuccessCodes 和其他Client实例。
// 返回新实例，原实例不受影响。
func (c *Client) WithSuccessCodes(codes ...int) *Client {
	cc := c.copy()
	cc.successCodes = make(map[int]struct{}, len(codes))
	for _, code := range codes {
		cc.successCodes[code] = struct{}{}
	}
	return cc
}

// WithMiddlewares 设置中间件链，按注册顺序从外到内执行。
// 每次调用替换整个中间件链（而非追加）。
// 返回新实例，原实例不受影响。
func (c *Client) WithMiddlewares(mws ...Middleware) *Client {
	cc := c.copy()
	cc.middlewares = append([]Middleware{}, mws...)
	return cc
}

// WithRetry 启用自动重试策略。
// maxAttempts 包含首次请求（如 maxAttempts=3 表示首次 + 2次重试）。
// baseInterval 为初始退避间隔，每次重试翻倍（指数退避）。
// shouldRetry 决定何时重试；为 nil 时默认对 5xx 或网络错误重试。
//
// 注意：仅对幂等方法（GET/HEAD/OPTIONS/DELETE）自动重试；
// 非幂等方法需要 shouldRetry 明确返回 true 才会重试。
// 返回新实例，原实例不受影响。
func (c *Client) WithRetry(maxAttempts int, baseInterval time.Duration, shouldRetry func(statusCode int, err error) bool) *Client {
	cc := c.copy()
	cc.retryConfig = &retryConfig{
		maxAttempts:  maxAttempts,
		baseInterval: baseInterval,
		shouldRetry:  shouldRetry,
	}
	return cc
}

// transport 返回Client的 *http.Transport。
// 若尚未设置Transport，会懒初始化一个基于 http.DefaultTransport 克隆的新 Transport。
// 若Transport已设置但非 *http.Transport 类型，返回nil（避免panic）。
func (c *Client) transport() *http.Transport {
	if c.httpClient.Transport == nil {
		if t, ok := http.DefaultTransport.(*http.Transport); ok {
			c.httpClient.Transport = t.Clone()
		} else {
			return nil
		}
	}
	if t, ok := c.httpClient.Transport.(*http.Transport); ok {
		return t
	}
	return nil
}

// isSuccessCode 判断给定状态码是否在成功码集合中。
func isSuccessCode(codes map[int]struct{}, code int) bool {
	if codes == nil {
		return false
	}
	_, ok := codes[code]
	return ok
}

// cloneMap 深拷贝一个 map[int]struct{}。
func cloneMap(m map[int]struct{}) map[int]struct{} {
	cloned := make(map[int]struct{}, len(m))
	for k := range m {
		cloned[k] = struct{}{}
	}
	return cloned
}

// sendRequest 发送HTTP请求，使用Client内部的httpClient（复用连接池）。
// 支持请求级选项（RequestOption），如 WithRequestSuccessCodes 覆盖本次成功码。
func (c *Client) sendRequest(ctx context.Context, method, urlStr string, body any, reqOpts ...RequestOption) *Response {
	// 构建请求级配置
	config := requestConfig{}
	for _, opt := range reqOpts {
		opt(&config)
	}

	// 优先返回 body 构建过程中的错误（如 JSON 序列化失败）
	if config.bodyErr != nil {
		return &Response{err: config.bodyErr}
	}

	// ---------- URL 处理：合并 query params ----------
	if len(config.queryParams) > 0 {
		parsed, err := url.Parse(urlStr)
		if err != nil {
			return &Response{
				err: util.WrapError("解析URL失败", err, "URL", urlStr),
			}
		}
		q := parsed.Query()
		for k, vs := range config.queryParams {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		parsed.RawQuery = q.Encode()
		urlStr = parsed.String()
	}

	// ---------- 超时处理 ----------
	if config.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.timeout)
		defer cancel()
	}

	// 确定本次请求使用的成功码：请求级 > Client级别
	codes := c.successCodes
	if config.successCodes != nil {
		codes = config.successCodes
	}

	// ---------- 构建请求体 ----------
	// body 优先级：选项body > 参数body > nil
	var reqBody io.Reader
	hasBody := false

	if config.body != nil {
		reqBody = bytes.NewReader(config.body)
		hasBody = true
	} else {
		switch body.(type) {
		case io.Reader:
			reqBody = body.(io.Reader)
			if reqBody != nil {
				hasBody = true
			}
		default:
			if body != nil {
				jsonData, err := json.Marshal(body)
				if err != nil {
					return &Response{
						err: fmt.Errorf("请求体JSON序列化失败: %w", err),
					}
				}
				reqBody = bytes.NewReader(jsonData)
				hasBody = true
			}
		}
	}

	// ---------- 创建HTTP请求 ----------
	req, err := http.NewRequestWithContext(ctx, method, urlStr, reqBody)
	if err != nil {
		return &Response{
			err: fmt.Errorf("创建请求失败: %w", err),
		}
	}

	// ---------- Content-Type 设置 ----------
	// 优先级：选项指定的 contentType > HeaderSetter 设置的头 > 默认 application/json
	if config.contentType != "" {
		req.Header.Set("Content-Type", config.contentType)
	}

	// 设置请求头（HeaderSetter 可以覆盖选项中的 Content-Type）
	if c.headerSetter != nil {
		c.headerSetter.SetHeaders(req)
	}

	// 默认 Content-Type（仅当有 body 且未设置任何 Content-Type 时）
	if hasBody && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 追加请求级 headers（不覆盖 HeaderSetter 已设置的）
	for k, vs := range config.headers {
		for _, v := range vs {
			if req.Header.Get(k) == "" {
				req.Header.Set(k, v)
			}
		}
	}

	// 设置请求级 cookies
	for _, cookie := range config.cookies {
		req.AddCookie(cookie)
	}

	// ---------- 发送请求（支持中间件链 + 重试） ----------
	var resp *http.Response

	// 构建中间件链
	if len(c.middlewares) > 0 || c.retryConfig != nil {
		// 最内层：实际 Transport
		var inner http.RoundTripper
		if t := c.transport(); t != nil {
			inner = t
		} else {
			inner = http.DefaultTransport
		}

		// 重试中间件（位于实际 Transport 之前）
		if c.retryConfig != nil {
			inner = retryMiddleware(c.retryConfig, method)(inner)
		}

		// 用户中间件（从外到内包装）
		finalTransport := inner
		for i := len(c.middlewares) - 1; i >= 0; i-- {
			finalTransport = c.middlewares[i](finalTransport)
		}

		// 使用临时 Client，替换 Transport
		tempClient := *c.httpClient
		tempClient.Transport = finalTransport
		resp, err = tempClient.Do(req)
	} else {
		resp, err = c.httpClient.Do(req)
	}
	if err != nil {
		return &Response{
			err: fmt.Errorf("发送请求失败: %w", err),
		}
	}
	defer resp.Body.Close()

	// ---------- 读取响应体 ----------
	bytesBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Response{
			err:        fmt.Errorf("读取响应体失败: %w", err),
			statusCode: resp.StatusCode,
			header:     resp.Header.Clone(),
		}
	}

	// ---------- 检查状态码 ----------
	if !isSuccessCode(codes, resp.StatusCode) {
		return &Response{
			body:       bytesBody,
			err:        fmt.Errorf("请求失败，状态码: %d，响应体: %s", resp.StatusCode, string(bytesBody)),
			statusCode: resp.StatusCode,
			header:     resp.Header.Clone(),
		}
	}

	return &Response{
		body:       bytesBody,
		err:        nil,
		statusCode: resp.StatusCode,
		header:     resp.Header.Clone(),
	}
}

// -------------------- API 接口实现 --------------------

// Get 发送GET请求，使用 context.Background()。
func (c *Client) Get(url string) *Response {
	return c.GetCtx(context.Background(), url)
}

// Post 发送POST请求，使用 context.Background()。
func (c *Client) Post(url string, body any) *Response {
	return c.PostCtx(context.Background(), url, body)
}

// Put 发送PUT请求，使用 context.Background()。
func (c *Client) Put(url string, body any) *Response {
	return c.PutCtx(context.Background(), url, body)
}

// Patch 发送PATCH请求，使用 context.Background()。
func (c *Client) Patch(url string, body any) *Response {
	return c.PatchCtx(context.Background(), url, body)
}

// Delete 发送DELETE请求，使用 context.Background()。
func (c *Client) Delete(url string, body any) *Response {
	return c.DeleteCtx(context.Background(), url, body)
}

// GetCtx 发送GET请求，支持外部context.Context。
func (c *Client) GetCtx(ctx context.Context, url string) *Response {
	return c.sendRequest(ctx, http.MethodGet, url, nil)
}

// PostCtx 发送POST请求，支持外部context.Context。
func (c *Client) PostCtx(ctx context.Context, url string, body any) *Response {
	return c.sendRequest(ctx, http.MethodPost, url, body)
}

// PutCtx 发送PUT请求，支持外部context.Context。
func (c *Client) PutCtx(ctx context.Context, url string, body any) *Response {
	return c.sendRequest(ctx, http.MethodPut, url, body)
}

// PatchCtx 发送PATCH请求，支持外部context.Context。
func (c *Client) PatchCtx(ctx context.Context, url string, body any) *Response {
	return c.sendRequest(ctx, http.MethodPatch, url, body)
}

// DeleteCtx 发送DELETE请求，支持外部context.Context。
func (c *Client) DeleteCtx(ctx context.Context, url string, body any) *Response {
	return c.sendRequest(ctx, http.MethodDelete, url, body)
}

// Do 发送任意HTTP方法的请求，使用 context.Background()。
// 支持请求级选项（如 WithRequestSuccessCodes）。
func (c *Client) Do(method, url string, body any, opts ...RequestOption) *Response {
	return c.DoCtx(context.Background(), method, url, body, opts...)
}

// DoCtx 发送任意HTTP方法的请求，支持外部context.Context和请求级选项。
func (c *Client) DoCtx(ctx context.Context, method, url string, body any, opts ...RequestOption) *Response {
	return c.sendRequest(ctx, method, url, body, opts...)
}
