package xhttp

import "net/http"

// 定义请求头设置接口
type HeaderSetter interface {
	// SetHeaders 为请求设置头信息
	SetHeaders(req *http.Request)
}
