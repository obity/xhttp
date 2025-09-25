package util

import "fmt"

// 包装错误信息
func WrapError(msg string, err error, details ...interface{}) error {
	errMsg := msg
	if err != nil {
		errMsg += ": " + err.Error()
	}
	// 拼接额外信息（如状态码、响应体）
	for i := 0; i < len(details); i += 2 {
		if i+1 < len(details) {
			errMsg += fmt.Sprintf("，%s: %v", details[i], details[i+1])
		}
	}
	return &ParseError{message: errMsg, original: err}
}

// 自定义解析错误类型
type ParseError struct {
	message  string
	original error
}

func (e *ParseError) Error() string { return e.message }
func (e *ParseError) Unwrap() error { return e.original }
