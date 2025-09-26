# xhttp
A powerful and easy-to-use HTTP client library.


## API

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
}
```

# Example


1. 上传文件header设置函数举例

```go

// 专门为multipart文件上传设置header函数
func NewUploadHeaderSetter(formDataContentType string) *UploadHeaderSetter {
	return &UploadHeaderSetter{multipartContentType: formDataContentType}
}

// 为文件上传定义专用的请求头设置器（动态设置multipart的Content-Type）
type UploadHeaderSetter struct {
	multipartContentType string // 存储multipart的Content-Type（包含边界）
}

func (h *UploadHeaderSetter) SetHeaders(req *http.Request) {
	req.Header.Set("accesstoken", "AccessToken")
	req.Header.Set("x-acgw-identity", "XAgwIdentity")
	//设置Content-Type为multipart专用类型
	req.Header.Set("Content-Type", h.multipartContentType)
}

```

2. 通用header设置函数举例
```go
// 通用header设置函数
func NewHeaders() *Headers {
	return new(Headers)
}

type Headers struct{}

func (h *Headers) SetHeaders(req *http.Request) {
	req.Header.Set("accesstoken", "AccessToken")
	req.Header.Set("x-acgw-identity", "XAgwIdentity")
	req.Header.Set("Content-Type", "application/json")
}
```

3. http请求举例

```

// 保持事件
func SaveEvent(eventDo *FeishuEventDo) error {
	url := ConsoleBaseAPI + "/feishu_event"
	var response FeishuEventDo
	headerSetter := NewConsoleHeaders()
	api := xhttp.NewAPI(headerSetter)
	return api.Post(url, eventDo).ParseJSON(&response)
}


// 修改事件为需要二次处理的状态 nextStep
func ChangeToNextStep(uuid, correlationId, feishuNo string, totalAmount float64) error {
	request := struct {
		CorrelationId string  `json:"correlationId,omitempty" bson:"correlationId,omitempty"`
		Status        string  `json:"status,omitempty" bson:"status,omitempty"`
		FeishuNo      string  `json:"feishuNo,omitempty" bson:"feishuNo,omitempty"`
		TotalAmount   float64 `json:"totalAmount" bson:"totalAmount"`
	}{CorrelationId: correlationId,
		Status:      "nextStep",
		FeishuNo:    feishuNo,
		TotalAmount: totalAmount,
	}
	url := ConsoleBaseAPI + "/feishu_event/" + uuid
	headerSetter := NewConsoleHeaders()
	api := xhttp.NewAPI(headerSetter)
	_, err := api.Patch(url, request).ParseBytes()
	if err != nil {
		return err
	}
	return nil
}

func eventList(status string) (*[]FeishuEventDo, error) {
	var err error
	params := url.Values{}
	params.Add("search", "event:=approve status:="+status)
	params.Add("limit", "10000")
	params.Add("sort", "creationTime")
	url := ConsoleBaseAPI + "/feishu_event?" + params.Encode()
	result := new(Response)
	headerSetter := NewConsoleHeaders()
	api := xhttp.NewAPI(headerSetter)
	err = api.Get(url).ParseJSON(&result)
	if err != nil {
		return nil, fmt.Errorf("查询状态= %v 待处理列表失败: %v", status, err)
	}
	return &result.Result, nil
}
```