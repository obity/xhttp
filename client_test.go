package xhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// -------------------- Builder 测试 --------------------

// TestNewClient 验证默认Client的初始状态
func TestNewClient(t *testing.T) {
	c := NewClient(nil)
	if c.httpClient == nil {
		t.Fatal("NewClient: httpClient 不应为 nil")
	}
	if len(c.successCodes) != 2 {
		t.Fatalf("NewClient: 默认成功码数量应为 2，实际为 %d", len(c.successCodes))
	}
	if !isSuccessCode(c.successCodes, http.StatusOK) {
		t.Error("NewClient: 默认成功码应包含 200")
	}
	if !isSuccessCode(c.successCodes, http.StatusCreated) {
		t.Error("NewClient: 默认成功码应包含 201")
	}
}

// TestWithTimeout 验证 WithTimeout 正确设置 Timeout
func TestWithTimeout(t *testing.T) {
	c := NewClient(nil).WithTimeout(5 * time.Second)
	if c.httpClient.Timeout != 5*time.Second {
		t.Errorf("WithTimeout: 期望 %v，实际 %v", 5*time.Second, c.httpClient.Timeout)
	}
}

// TestWithTransport 验证自定义 Transport 正确设置
func TestWithTransport(t *testing.T) {
	transport := &http.Transport{
		MaxIdleConns: 42,
	}
	c := NewClient(nil).WithTransport(transport)
	if c.httpClient.Transport != transport {
		t.Error("WithTransport: Transport 未正确设置")
	}
	// 验证 transport() 返回正确的类型
	if tp := c.transport(); tp == nil {
		t.Error("WithTransport: transport() 返回 nil")
	} else if tp.MaxIdleConns != 42 {
		t.Errorf("WithTransport: MaxIdleConns 期望 42，实际 %d", tp.MaxIdleConns)
	}
}

// TestWithMaxIdleConns 验证 MaxIdleConns 通过懒初始化 Transport 正确设置
func TestWithMaxIdleConns(t *testing.T) {
	c := NewClient(nil).WithMaxIdleConns(100)
	if tp := c.transport(); tp == nil {
		t.Fatal("WithMaxIdleConns: transport() 返回 nil")
	} else if tp.MaxIdleConns != 100 {
		t.Errorf("WithMaxIdleConns: 期望 %d，实际 %d", 100, tp.MaxIdleConns)
	}
}

// TestWithMaxIdleConns_nonTransport 验证当 Transport 不是 *http.Transport 时不 panic
func TestWithMaxIdleConns_nonTransport(t *testing.T) {
	c := NewClient(nil).WithTransport(&customRoundTripper{})
	// 不应 panic
	c.WithMaxIdleConns(100)
	// transport() 应返回 nil
	if tp := c.transport(); tp != nil {
		t.Error("WithMaxIdleConns_nonTransport: 非 *http.Transport 时 transport() 应返回 nil")
	}
}

// TestWithMaxIdleConnsPerHost 验证 MaxIdleConnsPerHost 正确设置
func TestWithMaxIdleConnsPerHost(t *testing.T) {
	c := NewClient(nil).WithMaxIdleConnsPerHost(10)
	if tp := c.transport(); tp == nil {
		t.Fatal("WithMaxIdleConnsPerHost: transport() 返回 nil")
	} else if tp.MaxIdleConnsPerHost != 10 {
		t.Errorf("WithMaxIdleConnsPerHost: 期望 %d，实际 %d", 10, tp.MaxIdleConnsPerHost)
	}
}

// TestWithIdleConnTimeout 验证 IdleConnTimeout 正确设置
func TestWithIdleConnTimeout(t *testing.T) {
	c := NewClient(nil).WithIdleConnTimeout(30 * time.Second)
	if tp := c.transport(); tp == nil {
		t.Fatal("WithIdleConnTimeout: transport() 返回 nil")
	} else if tp.IdleConnTimeout != 30*time.Second {
		t.Errorf("WithIdleConnTimeout: 期望 %v，实际 %v", 30*time.Second, tp.IdleConnTimeout)
	}
}

// TestWithSuccessCodes 验证 Client 级别成功码替换
func TestWithSuccessCodes(t *testing.T) {
	c := NewClient(nil).WithSuccessCodes(http.StatusAccepted, http.StatusNoContent)
	if len(c.successCodes) != 2 {
		t.Fatalf("WithSuccessCodes: 成功码数量应为 2，实际 %d", len(c.successCodes))
	}
	if !isSuccessCode(c.successCodes, http.StatusAccepted) {
		t.Error("WithSuccessCodes: 应包含 202")
	}
	if !isSuccessCode(c.successCodes, http.StatusNoContent) {
		t.Error("WithSuccessCodes: 应包含 204")
	}
	if isSuccessCode(c.successCodes, http.StatusOK) {
		t.Error("WithSuccessCodes: 不应包含 200（被替换）")
	}
	// 验证默认包变量未被修改
	if !isSuccessCode(defaultSuccessCodes, http.StatusOK) {
		t.Error("WithSuccessCodes: 不应修改 defaultSuccessCodes")
	}
}

// TestBuilderCopyOnWrite 验证 Builder 方法返回新实例，原实例不受影响
func TestBuilderCopyOnWrite(t *testing.T) {
	c1 := NewClient(nil)
	c2 := c1.WithTimeout(5 * time.Second)
	if c1 == c2 {
		t.Error("WithTimeout 应返回新实例")
	}
	if c1.httpClient.Timeout != 0 {
		t.Error("原实例不应被修改")
	}
	if c2.httpClient.Timeout != 5*time.Second {
		t.Error("新实例应包含新配置")
	}

	// 验证 WithTransport 也返回新实例
	c3 := c1.WithTransport(&http.Transport{})
	if c1 == c3 {
		t.Error("WithTransport 应返回新实例")
	}
	if c1.httpClient.Transport != nil {
		t.Error("原实例的 Transport 不应被修改")
	}

	// 验证 WithMaxIdleConns 不影响原实例
	c4 := c1.WithMaxIdleConns(100)
	if c1 == c4 {
		t.Error("WithMaxIdleConns 应返回新实例")
	}
	if c4.transport() == nil {
		t.Fatal("新实例 transport() 不应为 nil")
	}
	if c4.transport().MaxIdleConns != 100 {
		t.Errorf("新实例 MaxIdleConns 期望 %d，实际 %d", 100, c4.transport().MaxIdleConns)
	}
	// 原实例没有 Transport，c1.transport() 会 lazy-init 一个
	// 但那个 Transport 应该是 c1 私有的，不受 c4 影响
	t1 := c1.transport()
	t4 := c4.transport()
	if t1 == t4 {
		t.Error("原实例和新实例的 Transport 应该是不同实例")
	}
	if c1.httpClient.Transport == c4.httpClient.Transport {
		t.Error("原实例和新实例的 httpClient.Transport 应该是不同引用")
	}

	// 验证链式调用整体语义不变
	c5 := NewClient(nil).
		WithTimeout(10 * time.Second).
		WithMaxIdleConns(50)
	if c5.httpClient.Timeout != 10*time.Second {
		t.Errorf("链式调用 Timeout 期望 %v，实际 %v", 10*time.Second, c5.httpClient.Timeout)
	}
	if c5.transport().MaxIdleConns != 50 {
		t.Errorf("链式调用 MaxIdleConns 期望 %d，实际 %d", 50, c5.transport().MaxIdleConns)
	}
}

// TestBuilderChain 验证链式调用全部生效
func TestBuilderChain(t *testing.T) {
	c := NewClient(nil).
		WithTimeout(10 * time.Second).
		WithMaxIdleConns(50).
		WithMaxIdleConnsPerHost(5).
		WithIdleConnTimeout(15 * time.Second).
		WithSuccessCodes(http.StatusOK)

	if c.httpClient.Timeout != 10*time.Second {
		t.Errorf("BuilderChain Timeout: 期望 %v，实际 %v", 10*time.Second, c.httpClient.Timeout)
	}
	if tp := c.transport(); tp == nil {
		t.Fatal("BuilderChain: transport() 返回 nil")
	} else {
		if tp.MaxIdleConns != 50 {
			t.Errorf("BuilderChain MaxIdleConns: 期望 %d，实际 %d", 50, tp.MaxIdleConns)
		}
		if tp.MaxIdleConnsPerHost != 5 {
			t.Errorf("BuilderChain MaxIdleConnsPerHost: 期望 %d，实际 %d", 5, tp.MaxIdleConnsPerHost)
		}
		if tp.IdleConnTimeout != 15*time.Second {
			t.Errorf("BuilderChain IdleConnTimeout: 期望 %v，实际 %v", 15*time.Second, tp.IdleConnTimeout)
		}
	}
	if len(c.successCodes) != 1 || !isSuccessCode(c.successCodes, http.StatusOK) {
		t.Error("BuilderChain: successCodes 应仅包含 200")
	}
}

// -------------------- 请求与状态码测试 --------------------

// TestSendRequest_success 验证 200 响应成功
func TestSendRequest_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Errorf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	body, err := resp.ParseString()
	if err != nil {
		t.Fatalf("ParseString 失败: %v", err)
	}
	if *body != `{"status":"ok"}` {
		t.Errorf("响应体: 期望 %q，实际 %q", `{"status":"ok"}`, *body)
	}
}

// TestSendRequest_defaultCodes 验证默认成功码（200/201 成功，202 失败）
func TestSendRequest_defaultCodes(t *testing.T) {
	c := NewClient(nil)

	// 202 Accepted 应失败（不在默认成功码中）
	server202 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`accepted`))
	}))
	defer server202.Close()

	resp := c.Get(server202.URL)
	if resp.StatusCode() != http.StatusAccepted {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusAccepted, resp.StatusCode())
	}
	// 非成功码时 err 不为 nil
	if _, err := resp.ParseString(); err == nil {
		t.Error("202 应返回错误")
	}
}

// TestSendRequest_customClientCodes 验证 WithSuccessCodes 使 202 成功、200 失败
func TestSendRequest_customClientCodes(t *testing.T) {
	c := NewClient(nil).WithSuccessCodes(http.StatusAccepted, http.StatusNoContent)
	bodyText := "custom ok"

	// 202 Accepted 应成功
	server202 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(bodyText))
	}))
	defer server202.Close()

	resp := c.Get(server202.URL)
	if resp.StatusCode() != http.StatusAccepted {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusAccepted, resp.StatusCode())
	}
	str, err := resp.ParseString()
	if err != nil {
		t.Fatalf("202 应解析成功，但收到错误: %v", err)
	}
	if *str != bodyText {
		t.Errorf("响应体: 期望 %q，实际 %q", bodyText, *str)
	}

	// 200 OK 应失败（已被替换）
	server200 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server200.Close()

	resp2 := c.Get(server200.URL)
	if resp2.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp2.StatusCode())
	}
	if _, err := resp2.ParseString(); err == nil {
		t.Error("200 应返回错误（已被替换出成功码集合）")
	}
}

// TestSendRequest_requestLevelCodes 验证请求级成功码覆盖 Client 级别
func TestSendRequest_requestLevelCodes(t *testing.T) {
	c := NewClient(nil) // 默认只有 200/201

	server203 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNonAuthoritativeInfo) // 203
		w.Write([]byte(`non-authoritative`))
	}))
	defer server203.Close()

	// 使用请求级成功码覆盖：203 应成功
	resp := c.DoCtx(context.Background(), http.MethodGet, server203.URL, nil,
		WithRequestSuccessCodes(http.StatusNonAuthoritativeInfo))
	if resp.StatusCode() != http.StatusNonAuthoritativeInfo {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusNonAuthoritativeInfo, resp.StatusCode())
	}
	str, err := resp.ParseString()
	if err != nil {
		t.Fatalf("请求级成功码 203 应解析成功，但收到错误: %v", err)
	}
	if *str != "non-authoritative" {
		t.Errorf("响应体: 期望 %q，实际 %q", "non-authoritative", *str)
	}
}

// TestSendRequest_nonSuccess 验证 404 返回错误
func TestSendRequest_nonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`not found`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusNotFound {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusNotFound, resp.StatusCode())
	}
	if _, err := resp.ParseString(); err == nil {
		t.Error("404 应返回错误")
	}
}

// TestSendRequest_networkError 验证无法连接时返回网络错误
func TestSendRequest_networkError(t *testing.T) {
	c := NewClient(nil)
	// 使用一个不会响应的地址
	resp := c.Get("http://127.0.0.1:1")
	if resp.StatusCode() != 0 {
		t.Errorf("网络错误时 StatusCode 应为 0，实际 %d", resp.StatusCode())
	}
	if _, err := resp.ParseString(); err == nil {
		t.Error("网络错误时应返回错误")
	}
}

// TestSendRequest_contextTimeout 验证 Context 超时返回错误
func TestSendRequest_contextTimeout(t *testing.T) {
	// 创建一个会延迟响应的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`too late`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	c := NewClient(nil)
	resp := c.GetCtx(ctx, server.URL)
	if resp.StatusCode() != 0 {
		t.Errorf("超时时 StatusCode 应为 0，实际 %d", resp.StatusCode())
	}
	if _, err := resp.ParseString(); err == nil {
		t.Error("超时应返回错误")
	}
}

// -------------------- API 接口兼容性测试 --------------------

// TestNewAPI_returnsAPI 验证 NewAPI 返回值满足 API 接口
func TestNewAPI_returnsAPI(t *testing.T) {
	var api API = NewAPI(nil)
	if api == nil {
		t.Fatal("NewAPI 返回 nil")
	}
}

// TestNewAPI_allMethods 验证调用全部 10 个方法类型正确
func TestNewAPI_allMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	api := NewAPI(nil)

	// 测试全部 10 个方法
	tests := []struct {
		name string
		fn   func() *Response
	}{
		{"Get", func() *Response { return api.Get(server.URL) }},
		{"Post", func() *Response { return api.Post(server.URL, nil) }},
		{"Put", func() *Response { return api.Put(server.URL, nil) }},
		{"Patch", func() *Response { return api.Patch(server.URL, nil) }},
		{"Delete", func() *Response { return api.Delete(server.URL, nil) }},
		{"GetCtx", func() *Response { return api.GetCtx(context.Background(), server.URL) }},
		{"PostCtx", func() *Response { return api.PostCtx(context.Background(), server.URL, nil) }},
		{"PutCtx", func() *Response { return api.PutCtx(context.Background(), server.URL, nil) }},
		{"PatchCtx", func() *Response { return api.PatchCtx(context.Background(), server.URL, nil) }},
		{"DeleteCtx", func() *Response { return api.DeleteCtx(context.Background(), server.URL, nil) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.fn()
			if resp == nil {
				t.Fatal("返回的 Response 为 nil")
			}
			if resp.StatusCode() != http.StatusOK {
				t.Errorf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
			}
		})
	}
}

// -------------------- 请求体与 Content-Type 测试 --------------------

// TestRequestBody_json 验证 body 序列化为 JSON 且 Content-Type 默认 application/json
func TestRequestBody_json(t *testing.T) {
	var (
		mu        sync.Mutex
		gotBody   string
		gotCT     string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	c := NewClient(nil)
	resp := c.Post(server.URL, payload{Name: "test", Value: 42})
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}

	mu.Lock()
	defer mu.Unlock()

	if gotCT != "application/json" {
		t.Errorf("Content-Type: 期望 %q，实际 %q", "application/json", gotCT)
	}
	expectedBody := `{"name":"test","value":42}`
	if gotBody != expectedBody {
		t.Errorf("请求体: 期望 %q，实际 %q", expectedBody, gotBody)
	}
}

// TestRequestBody_ioReader 验证 io.Reader body 原样传递
func TestRequestBody_ioReader(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	rawBody := "raw io.Reader data"
	c := NewClient(nil)
	resp := c.Post(server.URL, strings.NewReader(rawBody))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}

	mu.Lock()
	defer mu.Unlock()

	if gotBody != rawBody {
		t.Errorf("请求体: 期望 %q，实际 %q", rawBody, gotBody)
	}
}

// TestRequestBody_nil 验证 nil body 不设 Content-Type
func TestRequestBody_nil(t *testing.T) {
	var gotCT string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}

	if gotCT != "" {
		t.Errorf("GET 请求无 body 时 Content-Type 应为空，实际 %q", gotCT)
	}
}

// TestRequestBody_ioReaderContentType 验证 io.Reader body 默认也有 application/json Content-Type
func TestRequestBody_ioReaderContentType(t *testing.T) {
	var gotCT string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	// io.Reader body 不为 nil，默认应设 Content-Type: application/json
	resp := c.Post(server.URL, strings.NewReader("raw"))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}

	if gotCT != "application/json" {
		t.Errorf("io.Reader body 时 Content-Type 默认应为 %q，实际 %q", "application/json", gotCT)
	}
}

// TestHeaderSetter_injection 验证 HeaderSetter 的 SetHeaders 被正确调用
func TestHeaderSetter_injection(t *testing.T) {
	var (
		mu      sync.Mutex
		gotAuth string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	setter := &testHeaderSetter{token: "Bearer test-token"}
	c := NewClient(setter)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}

	mu.Lock()
	defer mu.Unlock()

	expected := "Bearer test-token"
	if gotAuth != expected {
		t.Errorf("Authorization header: 期望 %q，实际 %q", expected, gotAuth)
	}
}

// TestClient_reuse 验证 Client 可复用（连接池不重复创建）
func TestClient_reuse(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	// 发送两次请求验证 Client 复用
	for i := 0; i < 3; i++ {
		resp := c.Get(server.URL)
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("第 %d 次请求失败: StatusCode=%d", i+1, resp.StatusCode())
		}
	}
	if requestCount != 3 {
		t.Errorf("服务器应收到 3 个请求，实际 %d", requestCount)
	}
}

// TestDo_method 验证 Do 方法支持任意 HTTP 方法
func TestDo_method(t *testing.T) {
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)

	// 测试 HEAD、OPTIONS 等非标准方法
	resp := c.Do(http.MethodHead, server.URL, nil)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotMethod != http.MethodHead {
		t.Errorf("Do 方法: 期望 %q，实际 %q", http.MethodHead, gotMethod)
	}
}

// TestIsSuccessCode_edgeCases 验证 isSuccessCode 边界情况
func TestIsSuccessCode_edgeCases(t *testing.T) {
	if isSuccessCode(nil, 200) {
		t.Error("nil 集合应返回 false")
	}
	if isSuccessCode(map[int]struct{}{}, 200) {
		t.Error("空集合应返回 false")
	}
}

// TestDefaultSuccessCodes_immutable 验证 defaultSuccessCodes 不被 WithSuccessCodes 修改
func TestDefaultSuccessCodes_immutable(t *testing.T) {
	// 先保存默认集合的快照
	originalSize := len(defaultSuccessCodes)
	originalHas200 := isSuccessCode(defaultSuccessCodes, http.StatusOK)

	// 修改 Client 的成功码
	_ = NewClient(nil).WithSuccessCodes(http.StatusAccepted)

	// 验证默认集合未被修改
	if len(defaultSuccessCodes) != originalSize {
		t.Errorf("defaultSuccessCodes 大小被修改: 原 %d，现 %d", originalSize, len(defaultSuccessCodes))
	}
	if isSuccessCode(defaultSuccessCodes, http.StatusOK) != originalHas200 {
		t.Error("defaultSuccessCodes 包含 200 的状态被修改")
	}
}

// TestClient_goroutineSafe 验证 Client 在并发下安全使用
func TestClient_goroutineSafe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := c.Get(server.URL)
			if resp.StatusCode() != http.StatusOK {
				errCh <- fmt.Errorf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
			}
			if _, err := resp.ParseString(); err != nil {
				errCh <- fmt.Errorf("ParseString 失败: %v", err)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

// -------------------- 测试辅助类型 --------------------

// customRoundTripper 实现 http.RoundTripper 但不使用 *http.Transport
type customRoundTripper struct{}

func (c *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(req)
}

// testHeaderSetter 用于测试 HeaderSetter 注入
type testHeaderSetter struct {
	token string
}

func (h *testHeaderSetter) SetHeaders(req *http.Request) {
	req.Header.Set("Authorization", h.token)
}

// testHeaderSetterWithCT 测试 HeaderSetter 覆盖 Content-Type
type testHeaderSetterWithCT struct{}

func (h *testHeaderSetterWithCT) SetHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/override")
}

// -------------------- P1: 请求级别配置测试 --------------------

// TestWithQueryParams 验证 query params 正确追加到 URL
func TestWithQueryParams(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodGet, server.URL, nil,
		WithQueryParams(map[string]string{"key": "value", "page": "1"}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotQuery != "key=value&page=1" && gotQuery != "page=1&key=value" {
		t.Errorf("Query: 期望包含 key=value&page=1，实际 %q", gotQuery)
	}
}

// TestWithQueryParams_existingQuery 验证 query params 与 URL 已有参数合并
func TestWithQueryParams_existingQuery(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodGet, server.URL+"?existing=1", nil,
		WithQueryParams(map[string]string{"key": "v"}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotQuery != "existing=1&key=v" {
		t.Errorf("Query: 期望 existing=1&key=v，实际 %q", gotQuery)
	}
}

// TestWithHeaders 验证请求级 headers 出现
func TestWithHeaders(t *testing.T) {
	var gotXTrace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXTrace = r.Header.Get("X-Trace-Id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodGet, server.URL, nil,
		WithHeaders(map[string]string{"X-Trace-Id": "abc-123"}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotXTrace != "abc-123" {
		t.Errorf("X-Trace-Id: 期望 %q，实际 %q", "abc-123", gotXTrace)
	}
}

// TestWithHeaders_headerSetterPriority 验证 HeaderSetter 优先级高于 WithHeaders
func TestWithHeaders_headerSetterPriority(t *testing.T) {
	var gotCT string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(&testHeaderSetterWithCT{})
	// 请求级 header 不应覆盖 HeaderSetter 设置的 Content-Type
	resp := c.DoCtx(context.Background(), http.MethodPost, server.URL, "body",
		WithHeaders(map[string]string{"Content-Type": "text/plain"}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotCT != "application/override" {
		t.Errorf("Content-Type: 期望 %q（HeaderSetter 优先级），实际 %q", "application/override", gotCT)
	}
}

// TestWithTimeout_requestLevel 验证请求级超时生效
func TestWithTimeout_requestLevel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	// 1μs 超时确保触发 deadline
	resp := c.DoCtx(context.Background(), http.MethodGet, server.URL, nil,
		WithTimeout(1*time.Microsecond))
	if resp.StatusCode() != 0 {
		t.Errorf("超时时 StatusCode 应为 0，实际 %d", resp.StatusCode())
	}
}

// TestWithCookies 验证请求级 Cookie
func TestWithCookies(t *testing.T) {
	var gotCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodGet, server.URL, nil,
		WithCookies(&http.Cookie{Name: "session", Value: "abc"}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotCookie != "session=abc" {
		t.Errorf("Cookie: 期望 %q，实际 %q", "session=abc", gotCookie)
	}
}

// TestRequestOptions_combined 验证多个选项同时生效
func TestRequestOptions_combined(t *testing.T) {
	var (
		gotQuery string
		gotCT    string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodGet, server.URL, nil,
		WithQueryParams(map[string]string{"q": "1"}),
		WithHeaders(map[string]string{"X-Custom": "val"}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotQuery != "q=1" {
		t.Errorf("Query: 期望 q=1，实际 %q", gotQuery)
	}
	if gotCT != "" {
		t.Errorf("GET 无 body 时 Content-Type 应为空，实际 %q", gotCT)
	}
}

// TestWithRequestSuccessCodes_empty 验证空 successCodes 回退到 Client 级别
func TestWithRequestSuccessCodes_empty(t *testing.T) {
	c := NewClient(nil).WithSuccessCodes(http.StatusAccepted)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	// 空参数应回退到 Client 级别
	resp := c.DoCtx(context.Background(), http.MethodGet, server.URL, nil,
		WithRequestSuccessCodes()) // 无参数
	if resp.StatusCode() != http.StatusAccepted {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusAccepted, resp.StatusCode())
	}
	if _, err := resp.ParseString(); err != nil {
		t.Errorf("空 successCodes 应回退到 Client 级别（含 202），但收到错误: %v", err)
	}
}

// -------------------- P3 Body 编码测试 --------------------

// TestWithJSONBody 验证 WithJSONBody 发送正确 JSON
func TestWithJSONBody(t *testing.T) {
	var (
		gotBody string
		gotCT   string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodPost, server.URL, nil,
		WithJSONBody(map[string]interface{}{"key": "val"}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type: 期望 application/json，实际 %q", gotCT)
	}
	if gotBody != `{"key":"val"}` {
		t.Errorf("Body: 期望 %q，实际 %q", `{"key":"val"}`, gotBody)
	}
}

// TestWithJSONBody_marshalError 验证不可序列化值返回错误
func TestWithJSONBody_marshalError(t *testing.T) {
	c := NewClient(nil)
	// channel 不可 JSON 序列化
	resp := c.DoCtx(context.Background(), http.MethodPost, "http://example.com", nil,
		WithJSONBody(make(chan int)))
	if resp.StatusCode() != 0 {
		t.Errorf("序列化失败时 StatusCode 应为 0，实际 %d", resp.StatusCode())
	}
	if _, err := resp.ParseString(); err == nil {
		t.Error("序列化失败时应返回错误")
	}
}

// TestWithFormBody 验证表单 body 编码
func TestWithFormBody(t *testing.T) {
	var (
		gotBody string
		gotCT   string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodPost, server.URL, nil,
		WithFormBody(map[string][]string{"name": {"test"}}))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotCT != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type: 期望 application/x-www-form-urlencoded，实际 %q", gotCT)
	}
	expected := "name=test"
	if gotBody != expected {
		t.Errorf("Body: 期望 %q，实际 %q", expected, gotBody)
	}
}

// TestWithRawBody 验证原始 body
func TestWithRawBody(t *testing.T) {
	var (
		gotBody string
		gotCT   string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodPost, server.URL, nil,
		WithRawBody([]byte("raw data")))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotBody != "raw data" {
		t.Errorf("Body: 期望 %q，实际 %q", "raw data", gotBody)
	}
	// 原始 body 不设置 Content-Type，默认应为 application/json（因为有 body）
	if gotCT != "application/json" {
		t.Errorf("Content-Type: 默认应为 application/json，实际 %q", gotCT)
	}
}

// TestWithRawStringBody 验证原始字符串 body
func TestWithRawStringBody(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.DoCtx(context.Background(), http.MethodPost, server.URL, nil,
		WithRawStringBody("raw string"))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotBody != "raw string" {
		t.Errorf("Body: 期望 %q，实际 %q", "raw string", gotBody)
	}
}

// TestBodyPriority_optionOverParameter 验证选项 body 优先于参数 body
func TestBodyPriority_optionOverParameter(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	// 参数 body 为 "param-body"，但选项 body 为 "option-body"
	resp := c.DoCtx(context.Background(), http.MethodPost, server.URL, map[string]string{"param": "body"},
		WithRawStringBody("option-body"))
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotBody != "option-body" {
		t.Errorf("Body: 期望 'option-body'（选项优先），实际 %q", gotBody)
	}
}

// TestWithMultipartBody 验证 multipart body
func TestWithMultipartBody(t *testing.T) {
	var (
		gotCT string
		hasContent bool
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		hasContent = len(b) > 0 && strings.Contains(string(b), "field1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	mw := NewMultipartBody()
	mw.AddField("field1", "value1")
	resp := c.DoCtx(context.Background(), http.MethodPost, server.URL, nil, mw.Option())
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data;") {
		t.Errorf("Content-Type: 应以 multipart/form-data 开头，实际 %q", gotCT)
	}
	if !hasContent {
		t.Error("multipart body 应包含 field1")
	}
}

// -------------------- P3 响应头测试 --------------------

// TestResponseHeader 验证成功响应包含响应头
func TestResponseHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	h := resp.Header()
	if h.Get("X-Custom") != "value" {
		t.Errorf("Header X-Custom: 期望 %q，实际 %q", "value", h.Get("X-Custom"))
	}
	if h.Get("Content-Type") == "" {
		t.Error("Header Content-Type 不应为空")
	}
}

// TestResponseHeader_requestError 验证请求前错误时 Header 返回空 map
func TestResponseHeader_requestError(t *testing.T) {
	c := NewClient(nil)
	resp := c.Get("http://127.0.0.1:1")
	if resp.Header() == nil {
		t.Error("Header() 不应返回 nil")
	}
	if len(resp.Header()) != 0 {
		t.Errorf("请求前错误时 Header 应为空，实际长度 %d", len(resp.Header()))
	}
}

// TestResponseHeader_isolation 验证 Header() 返回 clone，修改不影响内部
func TestResponseHeader_isolation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "original")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.Get(server.URL)
	h1 := resp.Header()
	h2 := resp.Header()

	// 修改 h1 不应影响 h2
	h1.Set("X-Custom", "modified")
	if h2.Get("X-Custom") != "original" {
		t.Error("Header() 返回的 map 应是独立的 clone")
	}
}

// TestResponseHeader_nonSuccess 验证非成功响应也包含响应头
func TestResponseHeader_nonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Error", "true")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`not found`))
	}))
	defer server.Close()

	c := NewClient(nil)
	resp := c.Get(server.URL)
	if resp.Header().Get("X-Error") != "true" {
		t.Errorf("非成功响应中 Header X-Error: 期望 %q，实际 %q", "true", resp.Header().Get("X-Error"))
	}
}

// -------------------- P2: 中间件测试 --------------------

// TestMiddleware_order 验证中间件按注册顺序执行
func TestMiddleware_order(t *testing.T) {
	var order []string
	mu := sync.Mutex{}

	mw1 := Middleware(func(next http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			order = append(order, "mw1_before")
			mu.Unlock()
			resp, err := next.RoundTrip(req)
			mu.Lock()
			order = append(order, "mw1_after")
			mu.Unlock()
			return resp, err
		})
	})

	mw2 := Middleware(func(next http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			order = append(order, "mw2_before")
			mu.Unlock()
			resp, err := next.RoundTrip(req)
			mu.Lock()
			order = append(order, "mw2_after")
			mu.Unlock()
			return resp, err
		})
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := NewClient(nil).WithMiddlewares(mw1, mw2)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}

	expected := []string{"mw1_before", "mw2_before", "mw2_after", "mw1_after"}
	if len(order) != len(expected) {
		t.Fatalf("中间件执行顺序长度: 期望 %d，实际 %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("执行顺序[%d]: 期望 %q，实际 %q", i, v, order[i])
		}
	}
}

// TestMiddleware_modifyRequest 验证中间件可以修改请求
func TestMiddleware_modifyRequest(t *testing.T) {
	var gotHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Middleware")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	addHeaderMW := Middleware(func(next http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Middleware", "injected")
			return next.RoundTrip(req)
		})
	})

	c := NewClient(nil).WithMiddlewares(addHeaderMW)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	if gotHeader != "injected" {
		t.Errorf("X-Middleware: 期望 %q，实际 %q", "injected", gotHeader)
	}
}

// TestWithMiddlewares_copyOnWrite 验证中间件链在 Builder 中不可变
func TestWithMiddlewares_copyOnWrite(t *testing.T) {
	mw1 := Middleware(func(next http.RoundTripper) http.RoundTripper { return next })
	mw2 := Middleware(func(next http.RoundTripper) http.RoundTripper { return next })

	c1 := NewClient(nil).WithMiddlewares(mw1)
	c2 := c1.WithMiddlewares(mw2)

	if len(c1.middlewares) != 1 {
		t.Errorf("原实例中间件数量应为 1，实际 %d", len(c1.middlewares))
	}
	if len(c2.middlewares) != 1 {
		t.Errorf("新实例中间件数量应为 1，实际 %d", len(c2.middlewares))
	}
}

// -------------------- P2: 重试测试 --------------------

// TestRetry_maxAttempts 验证达到最大重试次数后停止
func TestRetry_maxAttempts(t *testing.T) {
	attempts := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`error`))
	}))
	defer server.Close()

	c := NewClient(nil).WithRetry(3, 1*time.Millisecond, nil)
	resp := c.Get(server.URL)

	mu.Lock()
	a := attempts
	mu.Unlock()

	if a != 3 {
		t.Errorf("请求次数应等于 maxAttempts=3，实际 %d", a)
	}
	if resp.StatusCode() != http.StatusInternalServerError {
		t.Errorf("StatusCode: 期望 %d，实际 %d", http.StatusInternalServerError, resp.StatusCode())
	}
}

// TestRetry_successOnRetry 验证重试后成功
func TestRetry_successOnRetry(t *testing.T) {
	attempts := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		current := attempts
		mu.Unlock()
		if current < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`retry`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`success`))
	}))
	defer server.Close()

	c := NewClient(nil).WithRetry(5, 1*time.Millisecond, nil)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}
	str, err := resp.ParseString()
	if err != nil {
		t.Fatalf("ParseString 失败: %v", err)
	}
	if *str != "success" {
		t.Errorf("响应体: 期望 %q，实际 %q", "success", *str)
	}

	mu.Lock()
	a := attempts
	mu.Unlock()
	if a != 3 {
		t.Errorf("应重试 2 次后成功（共 3 次），实际 %d", a)
	}
}

// TestRetry_nonIdempotent 验证 POST 不自动重试
func TestRetry_nonIdempotent(t *testing.T) {
	attempts := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`error`))
	}))
	defer server.Close()

	c := NewClient(nil).WithRetry(5, 1*time.Millisecond, nil)
	resp := c.Post(server.URL, "body")

	mu.Lock()
	a := attempts
	mu.Unlock()

	// POST 非幂等，不应重试
	if a != 1 {
		t.Errorf("POST 非幂等请求不应重试，实际重试 %d 次", a)
	}
	if resp.StatusCode() != http.StatusInternalServerError {
		t.Errorf("StatusCode: 期望 %d，实际 %d", http.StatusInternalServerError, resp.StatusCode())
	}
}

// TestRetry_customShouldRetry 验证自定义 shouldRetry 逻辑
func TestRetry_customShouldRetry(t *testing.T) {
	attempts := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		current := attempts
		mu.Unlock()
		if current < 2 {
			w.WriteHeader(http.StatusBadGateway) // 502
			w.Write([]byte(`bad gateway`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	// 自定义：仅 502 重试
	shouldRetry := func(statusCode int, err error) bool {
		return statusCode == http.StatusBadGateway
	}

	c := NewClient(nil).WithRetry(3, 1*time.Millisecond, shouldRetry)
	resp := c.Get(server.URL)
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("StatusCode: 期望 %d，实际 %d", http.StatusOK, resp.StatusCode())
	}

	mu.Lock()
	a := attempts
	mu.Unlock()
	if a != 2 {
		t.Errorf("应重试 1 次（共 2 次），实际 %d", a)
	}
}

// TestRetry_contextCancelled 验证 context 取消停止重试
func TestRetry_contextCancelled(t *testing.T) {
	attempts := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`error`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	c := NewClient(nil).WithRetry(10, 10*time.Millisecond, nil)

	// 异步发送请求，然后立即取消
	errCh := make(chan *Response, 1)
	go func() {
		errCh <- c.GetCtx(ctx, server.URL)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	resp := <-errCh
	mu.Lock()
	a := attempts
	mu.Unlock()

	if a >= 10 {
		t.Error("context 取消后不应完成所有重试")
	}
	if resp.StatusCode() != 0 {
		t.Errorf("context 取消后 StatusCode 应为 0，实际 %d", resp.StatusCode())
	}
}

// TestDo_interface 验证 Do/DoCtx 在 API 接口上可用
func TestDo_interface(t *testing.T) {
	var api API = NewClient(nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Allow", "GET, POST")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	// Do 方法通过接口调用
	resp := api.Do(http.MethodOptions, server.URL, nil)
	if resp.StatusCode() != http.StatusOK {
		t.Errorf("Do via API: StatusCode=%d", resp.StatusCode())
	}
	if resp.Header().Get("Allow") != "GET, POST" {
		t.Errorf("Do via API: Allow header 期望 'GET, POST'，实际 %q", resp.Header().Get("Allow"))
	}

	// DoCtx 方法通过接口调用
	resp2 := api.DoCtx(context.Background(), http.MethodOptions, server.URL, nil)
	if resp2.StatusCode() != http.StatusOK {
		t.Errorf("DoCtx via API: StatusCode=%d", resp2.StatusCode())
	}
}

// TestErrorWrapping_errorsIs 验证 %w 错误包装可被 errors.Is 检查
func TestErrorWrapping_errorsIs(t *testing.T) {
	// 无法连接的地址，触发网络错误
	c := NewClient(nil)
	resp := c.Get("http://127.0.0.1:1")
	if resp.StatusCode() != 0 {
		t.Fatalf("StatusCode: 期望 0，实际 %d", resp.StatusCode())
	}

	// %w 包装后应能通过 errors.Unwrap 获取原始错误
	if resp.err == nil {
		t.Fatal("err 不应为 nil")
	}
	errStr := resp.err.Error()
	if !strings.Contains(errStr, "发送请求失败") {
		t.Errorf("错误消息应包含 '%s'，实际 %q", "发送请求失败", errStr)
	}
}

// TestHeaderSetter_contentTypeOverride 验证 HeaderSetter 可覆盖 Content-Type
func TestHeaderSetter_contentTypeOverride(t *testing.T) {
	var gotCT string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	// HeaderSetter 设置非默认 Content-Type
	setter := &testHeaderSetter{token: "ignored"}
	c := &Client{
		headerSetter: setter,
		httpClient:   &http.Client{},
		successCodes: cloneMap(defaultSuccessCodes),
	}
	// 手工设置 header 覆盖
	c.headerSetter = &testHeaderSetterWithCT{}

	_ = c.Post(server.URL, "some body")
	if gotCT != "application/override" {
		t.Errorf("Content-Type: 期望 %q，实际 %q", "application/override", gotCT)
	}
}
