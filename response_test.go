package xhttp

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name         string
		body         []byte
		err          error
		result       interface{}
		wantErr      bool
		errSubstring string
	}{
		{
			name:    "成功解析",
			body:    []byte(`{"name":"test"}`),
			result:  &struct{ Name string `json:"name"` }{},
			wantErr: false,
		},
		{
			name:         "result 为 nil",
			body:         []byte(`{"name":"test"}`),
			result:       nil,
			wantErr:      true,
			errSubstring: "result 不能为 nil",
		},
		{
			name:         "非法 JSON",
			body:         []byte(`{"name":"test"`),
			result:       &struct{ Name string `json:"name"` }{},
			wantErr:      true,
			errSubstring: "JSON解析失败",
		},
		{
			name:         "请求过程中已报错",
			body:         []byte(`{"name":"test"}`),
			err:          errors.New("network error"),
			result:       &struct{ Name string `json:"name"` }{},
			wantErr:      true,
			errSubstring: "network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Response{
				body: tt.body,
				err:  tt.err,
			}
			err := r.ParseJSON(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errSubstring) {
				t.Errorf("ParseJSON() error = %v, want substring %v", err, tt.errSubstring)
			}
		})
	}
}

func TestParseXML(t *testing.T) {
	tests := []struct {
		name         string
		body         []byte
		err          error
		result       interface{}
		wantErr      bool
		errSubstring string
	}{
		{
			name:    "成功解析",
			body:    []byte(`<Test><Name>test</Name></Test>`),
			result:  &struct{ Name string `xml:"Name"` }{},
			wantErr: false,
		},
		{
			name:         "result 为 nil",
			body:         []byte(`<Test><Name>test</Name></Test>`),
			result:       nil,
			wantErr:      true,
			errSubstring: "result 不能为 nil",
		},
		{
			name:         "非法 XML",
			body:         []byte(`<Test><Name>test</Name>`),
			result:       &struct{ Name string `xml:"Name"` }{},
			wantErr:      true,
			errSubstring: "XML解析失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Response{
				body: tt.body,
				err:  tt.err,
			}
			err := r.ParseXML(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseXML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errSubstring) {
				t.Errorf("ParseXML() error = %v, want substring %v", err, tt.errSubstring)
			}
		})
	}
}

func TestParseProtobuf(t *testing.T) {
	// Use structpb for a real proto.Message
	msg, err := structpb.NewStruct(map[string]interface{}{
		"name": "test",
	})
	if err != nil {
		t.Fatalf("Failed to create structpb: %v", err)
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal structpb: %v", err)
	}

	tests := []struct {
		name         string
		body         []byte
		err          error
		result       proto.Message
		wantErr      bool
		errSubstring string
	}{
		{
			name:    "成功解析",
			body:    data,
			result:  &structpb.Struct{},
			wantErr: false,
		},
		{
			name:         "result 为 nil",
			body:         data,
			result:       nil,
			wantErr:      true,
			errSubstring: "result 不能为 nil",
		},
		{
			name:         "非法数据",
			body:         []byte{0xff, 0x00, 0xaa},
			result:       &structpb.Struct{},
			wantErr:      true,
			errSubstring: "Protobuf解析失败",
		},
		{
			name:         "请求过程中已报错",
			body:         data,
			err:          errors.New("network error"),
			result:       &structpb.Struct{},
			wantErr:      true,
			errSubstring: "network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Response{
				body: tt.body,
				err:  tt.err,
			}
			err := r.ParseProtobuf(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProtobuf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errSubstring) {
				t.Errorf("ParseProtobuf() error = %v, want substring %v", err, tt.errSubstring)
			}
			if !tt.wantErr && tt.name == "成功解析" && tt.result != nil {
				res := tt.result.(*structpb.Struct)
				if res.Fields["name"].GetStringValue() != "test" {
					t.Errorf("ParseProtobuf() expected name='test', got '%s'", res.Fields["name"].GetStringValue())
				}
			}
		})
	}
}

func TestParseBytes(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		err     error
		wantErr bool
	}{
		{
			name:    "成功获取",
			body:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "请求过程中已报错",
			body:    []byte("hello world"),
			err:     errors.New("network error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Response{
				body: tt.body,
				err:  tt.err,
			}
			got, err := r.ParseBytes()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != string(tt.body) {
				t.Errorf("ParseBytes() got = %v, want %v", got, tt.body)
			}
		})
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		err     error
		wantErr bool
	}{
		{
			name:    "成功获取",
			body:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "请求过程中已报错",
			body:    []byte("hello world"),
			err:     errors.New("network error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Response{
				body: tt.body,
				err:  tt.err,
			}
			got, err := r.ParseString()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && *got != string(tt.body) {
				t.Errorf("ParseString() got = %v, want %v", *got, string(tt.body))
			}
		})
	}
}
