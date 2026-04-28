package xhttp

import (
	"encoding/json"
	"encoding/xml"
	"net/http"

	"github.com/obity/xhttp/util"
	"google.golang.org/protobuf/proto"
)

// Response 封装HTTP响应结果，提供多种解析方法。
// 支持JSON、XML、Protobuf解析，以及原始字节和字符串获取。
type Response struct {
	body       []byte      // 响应的数据
	err        error       // 请求过程中的错误
	statusCode int         // HTTP状态码
	header     http.Header // 响应头（仅成功接收到HTTP响应时有值）
}

// StatusCode 返回HTTP状态码，方便调用者检测错误的不同类型
// 如果请求过程中发生错误（如网络错误、JSON序列化失败等），返回0
func (r *Response) StatusCode() int {
	return r.statusCode
}

// Header 返回响应头的深拷贝。
// 仅在能成功接收到HTTP响应时有值（此时响应头非空）。
// 在请求前错误（如网络错误、序列化失败）或请求成功但Body读取失败时也可能有值。
// 返回的 map 是响应头的拷贝，调用方可以安全修改。
func (r *Response) Header() http.Header {
	if r.header == nil {
		return http.Header{}
	}
	return r.header.Clone()
}

// 从响应中解析JSON
func (r *Response) ParseJSON(result interface{}) error {
	if r.err != nil {
		return r.err
	}
	if result == nil {
		return util.WrapError("解析失败", nil, "原因", "result 不能为 nil，需传入 JSON结构体")
	}
	if err := json.Unmarshal(r.body, result); err != nil {
		return util.WrapError("JSON解析失败", err, "响应体", string(r.body))
	}
	return nil
}

// 从响应中解析XML
func (r *Response) ParseXML(result interface{}) error {
	if r.err != nil {
		return r.err
	}
	if result == nil {
		return util.WrapError("解析失败", nil, "原因", "result 不能为 nil，需传入 XML结构体")
	}
	if err := xml.Unmarshal(r.body, result); err != nil {
		return util.WrapError("XML解析失败", err, "响应体", string(r.body))
	}
	return nil
}

// ParseProtobuf 解析Protobuf格式响应
// 注意：result必须是proto.Message接口的实现（如生成的pb结构体指针）
func (r *Response) ParseProtobuf(result proto.Message) error {
	if r.err != nil {
		return r.err
	}
	if result == nil {
		return util.WrapError("解析失败", nil, "原因", "result 不能为 nil，需传入 proto.Message")
	}
	// 解析Protobuf（依赖proto.Unmarshal）
	if err := proto.Unmarshal(r.body, result); err != nil {
		return util.WrapError("Protobuf解析失败", err,
			"响应体长度", len(r.body), // Protobuf是二进制，不适合打印原文
		)
	}
	return nil
}

// ParseBytes 将响应体作为二进制数据返回
// 注意：result 必须是 *[]byte 类型（如 &[]byte{}）
func (r *Response) ParseBytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.body, nil
}

// ParseString 将响应体转换为字符串返回
// 注意：result 必须是 *string 类型
func (r *Response) ParseString() (*string, error) {
	if r.err != nil {
		return nil, r.err
	}

	// 将字节转换为UTF-8字符串（二进制安全）
	result := string(r.body)
	return &result, nil
}
