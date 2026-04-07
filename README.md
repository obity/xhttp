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

## Response

```go
type Response struct {
    // 内部字段，不直接访问
}

// StatusCode 返回HTTP状态码，方便调用者检测错误的不同类型
// 如果请求过程中发生错误（如网络错误、JSON序列化失败等），返回0
func (r *Response) StatusCode() int

// ParseJSON 解析JSON响应
func (r *Response) ParseJSON(result interface{}) error

// ParseXML 解析XML响应
func (r *Response) ParseXML(result interface{}) error

// ParseProtobuf 解析Protobuf响应
func (r *Response) ParseProtobuf(result proto.Message) error

// ParseBytes 返回原始字节数据
func (r *Response) ParseBytes() ([]byte, error)

// ParseString 返回字符串数据
func (r *Response) ParseString() (*string, error)
```

## Example


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

4. 使用 StatusCode() 检测不同类型的错误

```go
func getUser(userId string) (*User, error) {
	url := APIBaseURL + "/users/" + userId
	headerSetter := NewHeaders()
	api := xhttp.NewAPI(headerSetter)
	resp := api.Get(url)
	
	// 检测HTTP状态码
	switch code := resp.StatusCode(); code {
	case http.StatusOK:
		// 成功，解析数据
		var user User
		if err := resp.ParseJSON(&user); err != nil {
			return nil, err
		}
		return &user, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("认证失败，请检查token")
	case http.StatusForbidden:
		return nil, fmt.Errorf("没有权限访问该资源")
	case http.StatusNotFound:
		return nil, fmt.Errorf("用户不存在: %s", userId)
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("服务器内部错误")
	default:
		if code == 0 {
			// 状态码为0表示请求发送前就失败了（网络错误、JSON序列化失败等）
			return nil, fmt.Errorf("请求失败: %v", resp.ParseJSON(new(interface{})))
		}
		return nil, fmt.Errorf("请求失败，状态码: %d", code)
	}
}
```