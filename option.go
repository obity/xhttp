package xhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/obity/xhttp/util"
)

// requestConfig 请求级配置
type requestConfig struct {
	successCodes map[int]struct{}
	queryParams  url.Values
	headers      http.Header
	timeout      time.Duration
	cookies      []*http.Cookie

	// body 相关（优先级高于 Do(url, body) 参数中的 body）
	body        []byte // 预编码的 body 数据
	bodyErr     error  // body 构建过程中的错误（延迟到 sendRequest 中返回）
	contentType string // 对应的 Content-Type（空表示由 sendRequest 决定）
}

// RequestOption 请求级选项函数，用于覆盖或补充Client级别的配置
type RequestOption func(*requestConfig)

// WithRequestSuccessCodes 设置请求级的成功状态码，覆盖Client级别的成功码设置
// 当某个请求需要特殊的成功判定规则时使用
func WithRequestSuccessCodes(codes ...int) RequestOption {
	return func(c *requestConfig) {
		if len(codes) == 0 {
			c.successCodes = nil // 无参数时使用Client级别配置
			return
		}
		c.successCodes = make(map[int]struct{}, len(codes))
		for _, code := range codes {
			c.successCodes[code] = struct{}{}
		}
	}
}

// ---------- P1: 请求级别配置 ----------

// WithQueryParams 设置请求级查询参数，追加到URL已有参数之上。
// 与URL中已有参数合并（而非覆盖），例如：
//
//	URL: "/api?page=1" + WithQueryParams(map[string]string{"page":"2"}) → "/api?page=1&page=2"
func WithQueryParams(params map[string]string) RequestOption {
	return func(c *requestConfig) {
		if c.queryParams == nil {
			c.queryParams = make(url.Values, len(params))
		}
		for k, v := range params {
			c.queryParams.Add(k, v) // 追加，即使同一key多次调用也追加
		}
	}
}

// WithHeaders 设置请求级HTTP头，追加到 HeaderSetter 和已有头之上。
// 注意：不会覆盖 HeaderSetter 已设置的同名头，也不会覆盖 Content-Type。
func WithHeaders(headers map[string]string) RequestOption {
	return func(c *requestConfig) {
		if c.headers == nil {
			c.headers = make(http.Header, len(headers))
		}
		for k, v := range headers {
			c.headers.Add(k, v)
		}
	}
}

// WithTimeout 设置请求级超时。当 timeout > 0 时，创建派生 context 覆盖 Client 级别的超时。
// 如果外部 context 已有更早的 deadline，以更早的为准。
func WithTimeout(timeout time.Duration) RequestOption {
	return func(c *requestConfig) {
		c.timeout = timeout
	}
}

// WithCookies 设置请求级 Cookie。
func WithCookies(cookies ...*http.Cookie) RequestOption {
	return func(c *requestConfig) {
		c.cookies = append(c.cookies, cookies...)
	}
}

// ---------- P3: Body 编码扩展 ----------

// WithJSONBody 将任意值序列化为JSON作为请求体，同时设置 Content-Type 为 application/json。
// 如果序列化失败，错误会在 sendRequest 时返回。
//
// 优先级高于 Do(url, body) 参数中的 body。
func WithJSONBody(v interface{}) RequestOption {
	return func(c *requestConfig) {
		data, err := json.Marshal(v)
		if err != nil {
			c.bodyErr = util.WrapError("JSON序列化请求体失败", err)
			return
		}
		c.body = data
		c.contentType = "application/json"
	}
}

// WithFormBody 将 url.Values 编码为 x-www-form-urlencoded 格式作为请求体，
// 同时设置 Content-Type 为 application/x-www-form-urlencoded。
func WithFormBody(values url.Values) RequestOption {
	return func(c *requestConfig) {
		c.body = []byte(values.Encode())
		c.contentType = "application/x-www-form-urlencoded"
	}
}

// WithRawBody 直接使用原始字节数组作为请求体，不设置 Content-Type。
// 可通过 HeaderSetter 或 WithHeaders 设置 Content-Type。
func WithRawBody(data []byte) RequestOption {
	return func(c *requestConfig) {
		c.body = data
		// 不设置 contentType，由 HeaderSetter 或 sendRequest 默认决定
	}
}

// WithRawStringBody 使用原始字符串作为请求体，不设置 Content-Type。
func WithRawStringBody(s string) RequestOption {
	return func(c *requestConfig) {
		c.body = []byte(s)
	}
}

// MultipartWriter 辅助构建 multipart/form-data 请求体。
//
// 使用示例：
//
//	mw := xhttp.NewMultipartBody()
//	mw.AddField("name", "value").
//		AddFile("file", "test.txt", bytes.NewReader(content))
//	resp := client.Do("POST", url, nil, mw.Option())
type MultipartWriter struct {
	buf  *bytes.Buffer
	mw   *multipart.Writer
	once sync.Once
	err  error
}

// NewMultipartBody 创建 MultipartWriter 实例。调用 Option() 获取 RequestOption。
func NewMultipartBody() *MultipartWriter {
	return &MultipartWriter{buf: new(bytes.Buffer)}
}

func (m *MultipartWriter) init() {
	m.once.Do(func() {
		m.mw = multipart.NewWriter(m.buf)
	})
}

// AddField 添加文本字段。
func (m *MultipartWriter) AddField(key, value string) *MultipartWriter {
	m.init()
	if m.err != nil {
		return m
	}
	if err := m.mw.WriteField(key, value); err != nil {
		m.err = util.WrapError("添加multipart字段失败", err)
	}
	return m
}

// AddFile 添加文件字段。
// fieldName 为表单字段名，filename 为文件名，r 为文件内容。
func (m *MultipartWriter) AddFile(fieldName, filename string, r io.Reader) *MultipartWriter {
	m.init()
	if m.err != nil {
		return m
	}
	w, err := m.mw.CreateFormFile(fieldName, filename)
	if err != nil {
		m.err = util.WrapError("创建multipart文件字段失败", err)
		return m
	}
	if _, err := io.Copy(w, r); err != nil {
		m.err = util.WrapError("写入multipart文件内容失败", err)
	}
	return m
}

// Option 返回 RequestOption，将构建好的 multipart body 和 Content-Type 注入请求。
// 必须在 AddField/AddFile 全部调用完成后调用一次。
func (m *MultipartWriter) Option() RequestOption {
	return func(c *requestConfig) {
		m.init()
		if m.err != nil {
			c.bodyErr = m.err
			return
		}
		if err := m.mw.Close(); err != nil {
			c.bodyErr = util.WrapError("关闭multipart writer失败", err)
			return
		}
		c.body = m.buf.Bytes()
		c.contentType = m.mw.FormDataContentType()
	}
}
