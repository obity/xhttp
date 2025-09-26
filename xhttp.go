package xhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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
}

func NewAPI(headerSetter HeaderSetter) API {
	return &apiImpl{setter: headerSetter}
}

type apiImpl struct {
	setter HeaderSetter
}

func (a *apiImpl) Get(url string) *Response {
	return a.GetCtx(context.Background(), url)
}

func (a *apiImpl) Post(url string, body any) *Response {
	return a.PostCtx(context.Background(), url, body)
}

func (a *apiImpl) Patch(url string, body any) *Response {
	return a.PatchCtx(context.Background(), url, body)
}

func (a *apiImpl) Put(url string, body any) *Response {
	return a.PutCtx(context.Background(), url, body)
}

func (a *apiImpl) Delete(url string, body any) *Response {
	return a.DeleteCtx(context.Background(), url, body)
}

func (a *apiImpl) GetCtx(ctx context.Context, url string) *Response {
	return sendRequest(ctx, http.MethodGet, url, nil, a.setter)
}

func (a *apiImpl) PostCtx(ctx context.Context, url string, body any) *Response {
	return sendRequest(ctx, http.MethodPost, url, body, a.setter)
}

func (a *apiImpl) PatchCtx(ctx context.Context, url string, body any) *Response {
	return sendRequest(ctx, http.MethodPatch, url, body, a.setter)
}

func (a *apiImpl) PutCtx(ctx context.Context, url string, body any) *Response {
	return sendRequest(ctx, http.MethodPut, url, body, a.setter)
}

func (a *apiImpl) DeleteCtx(ctx context.Context, url string, body any) *Response {
	return sendRequest(ctx, http.MethodDelete, url, body, a.setter)
}

// 发送HTTP请求的公共函数（每次创建新client）
func sendRequest(ctx context.Context, method, url string, body any, headerSetter HeaderSetter) *Response {
	// 构建请求体
	var reqBody io.Reader
	switch v := body.(type) {
	case io.Reader:
		// 直接使用io.Reader（如multipart.Reader）文件上传body不是json
		reqBody = v
	default:
		if body != nil {
			jsonData, err := json.Marshal(body)
			if err != nil {
				return &Response{
					body: nil,
					err:  fmt.Errorf("请求体JSON序列化失败: %v", err),
				}
			}
			reqBody = bytes.NewBuffer(jsonData)
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return &Response{
			body: nil,
			err:  fmt.Errorf("创建请求失败: %v", err),
		}
	}
	// 设置默认Content-Type（可被headerSetter覆盖）
	if reqBody != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	// 设置请求头
	if headerSetter != nil {
		headerSetter.SetHeaders(req)
	}

	client := &http.Client{}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return &Response{
			body: nil,
			err:  fmt.Errorf("发送请求失败: %v", err),
		}
	}
	defer resp.Body.Close()
	// 读取响应体
	bytesBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Response{
			body: nil,
			err:  fmt.Errorf("读取响应体失败: %v", err),
		}
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &Response{
			body: nil,
			err:  fmt.Errorf("请求失败，状态码: %d，响应体: %s", resp.StatusCode, string(bytesBody)),
		}
	}
	return &Response{
		body: bytesBody,
		err:  err,
	}

}
