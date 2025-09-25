package xhttp

import (
	"encoding/json"
	"encoding/xml"

	"github.com/obity/xhttp/util"
	"google.golang.org/protobuf/proto"
)

type Response struct {
	body []byte // 响应的数据
	err  error  // 请求过程中的错误
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
		return util.WrapError("请求失败", r.err)
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
func (r *Response) ParseBytes(result *[]byte) error {
	if r.err != nil {
		return r.err
	}
	if result == nil {
		return util.WrapError("解析失败", nil, "原因", "result 不能为 nil，需传入 *[]byte")
	}
	// 直接复制原始字节（避免外部修改影响内部数据）
	*result = make([]byte, len(r.body))
	copy(*result, r.body)
	return nil
}

// ParseString 将响应体转换为字符串返回
// 注意：result 必须是 *string 类型
func (r *Response) ParseString(result *string) error {
	if r.err != nil {
		return r.err
	}
	if result == nil {
		return util.WrapError("解析失败", nil, "原因", "result 不能为 nil，需传入 *string")
	}
	// 将字节转换为UTF-8字符串（二进制安全）
	*result = string(r.body)
	return nil
}
