# xhttp
一个强大且易用的 Go 语言 HTTP 客户端库，支持流式配置、自动重试、中间件扩展及多种响应解析。

## 特性

- **流式 API**: 支持 Builder 模式配置客户端。
- **请求级选项**: 灵活覆盖单次请求的 Timeout、Header、Query Params 等。
- **中间件支持**: 采用装饰器模式，支持日志、监控等扩展。
- **自动重试**: 内置指数退避重试机制，支持幂等性自动判断。
- **丰富的解析器**: 原生支持 JSON、XML、Protobuf 及原始字节/字符串解析。

## API 接口

```go
type API interface {
	Get(url string) *Response
	Post(url string, body any) *Response
	Patch(url string, body any) *Response
	Put(url string, body any) *Response
	Delete(url string, body any) *Response
	GetCtx(ctx context.Context, url string) *Response
	PostCtx(ctx context.Context, url string, body any) *Response
	PatchCtx(ctx context.Context, url string, body any) *Response
	PutCtx(ctx context.Context, url string, body any) *Response
	DeleteCtx(ctx context.Context, url string, body any) *Response
	// Do/DoCtx 支持请求级选项 (RequestOption)
	Do(method, url string, body any, opts ...RequestOption) *Response
	DoCtx(ctx context.Context, method, url string, body any, opts ...RequestOption) *Response
}
```

## Response 响应处理

```go
// StatusCode 返回HTTP状态码。请求前失败返回 0。
func (r *Response) StatusCode() int

// Header 返回响应头的克隆副本。
func (r *Response) Header() http.Header

// ParseJSON 解析 JSON 响应。
func (r *Response) ParseJSON(result interface{}) error

// ParseXML 解析 XML 响应。
func (r *Response) ParseXML(result interface{}) error

// ParseProtobuf 解析 Protobuf 响应。
func (r *Response) ParseProtobuf(result proto.Message) error

// ParseBytes 返回原始字节数据。
func (r *Response) ParseBytes() ([]byte, error)

// ParseString 返回字符串数据。
func (r *Response) ParseString() (*string, error)
```

## 快速开始

### 1. 基础请求与日志中间件

```go
func main() {
    // 创建客户端，并添加内置日志中间件
    api := xhttp.NewClient(nil).
        WithTimeout(10 * time.Second).
        WithMiddlewares(xhttp.LoggingMiddleware(nil))

    var result map[string]any
    err := api.Get("https://api.example.com/info").ParseJSON(&result)
    if err != nil {
        log.Fatal(err)
    }
}
```

### 2. 使用自动重试机制

```go
// 配置：最多尝试 3 次，初始退避 100ms，仅对 5xx 错误重试
api := xhttp.NewClient(nil).
    WithRetry(3, 100 * time.Millisecond, nil)

// GET 请求将自动重试幂等操作
resp := api.Get("https://api.example.com/unstable")
```

### 3. 请求级选项 (RequestOption)

```go
api := xhttp.NewAPI(nil)

// 使用 Do 方法配合选项
resp := api.Do("POST", "/upload", nil, 
    xhttp.WithQueryParams(map[string]string{"version": "v1"}),
    xhttp.WithHeaders(map[string]string{"X-Request-ID": "123"}),
    xhttp.WithJSONBody(map[string]string{"file": "data"}),
)
```

### 4. 文件上传 (Multipart)

```go
mw := xhttp.NewMultipartBody()
mw.AddField("name", "xhttp").
   AddFile("avatar", "logo.png", fileReader)

api := xhttp.NewAPI(nil)
err := api.Do("POST", "/user/avatar", nil, mw.Option()).ParseJSON(&res)
```

### 5. 自定义 HeaderSetter (全局逻辑)

```go
type MyAuth struct{}
func (a *MyAuth) SetHeaders(req *http.Request) {
    req.Header.Set("Authorization", "Bearer token")
}

api := xhttp.NewAPI(&MyAuth{})
```

## 贡献指南

1. **创建分支**: 基于 `master` 分支创建功能分支。
2. **测试**: 确保通过所有单元测试 `go test ./...`。
3. **提交**: 遵循项目规范，提交代码并合并回 `master`。
