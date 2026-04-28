package xhttp

import "context"

// API 定义了HTTP请求的统一接口。
// 提供5种HTTP方法的简写（Get/Post/Put/Patch/Delete）及其带Context的版本。
// 同时也支持通用 Do/DoCtx 方法配合请求级选项使用。
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
	Do(method, url string, body any, opts ...RequestOption) *Response
	DoCtx(ctx context.Context, method, url string, body any, opts ...RequestOption) *Response
}

// NewAPI 创建API接口实例。
//
// 返回的API实例底层使用 *Client，支持超时、Transport配置等高级功能。
// 调用方可通过类型断言获取 *Client 以使用Builder方法：
//
//	client := xhttp.NewAPI(setter).(*xhttp.Client)
//	client.WithTimeout(5 * time.Second)
func NewAPI(headerSetter HeaderSetter) API {
	return NewClient(headerSetter)
}
