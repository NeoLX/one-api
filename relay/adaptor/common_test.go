package adaptor

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/client"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/meta"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

type fakeAdaptor struct {
	url string
}

func (f *fakeAdaptor) Init(*meta.Meta)                          {}
func (f *fakeAdaptor) GetRequestURL(*meta.Meta) (string, error) { return f.url, nil }
func (f *fakeAdaptor) SetupRequestHeader(c *gin.Context, req *http.Request, m *meta.Meta) error {
	// 模拟真实适配器：先设通用头，再设自己的认证头
	SetupCommonRequestHeader(c, req, m)
	req.Header.Set("Authorization", "Bearer adaptor-default")
	req.Header.Set("X-Adaptor-Only", "yes")
	return nil
}
func (f *fakeAdaptor) ConvertRequest(*gin.Context, int, *relaymodel.GeneralOpenAIRequest) (any, error) {
	return nil, nil
}
func (f *fakeAdaptor) ConvertImageRequest(*relaymodel.ImageRequest) (any, error) { return nil, nil }
func (f *fakeAdaptor) DoRequest(c *gin.Context, m *meta.Meta, body io.Reader) (*http.Response, error) {
	return DoRequestHelper(f, c, m, body)
}
func (f *fakeAdaptor) DoResponse(*gin.Context, *http.Response, *meta.Meta) (*relaymodel.Usage, *relaymodel.ErrorWithStatusCode) {
	return nil, nil
}
func (f *fakeAdaptor) GetModelList() []string { return nil }
func (f *fakeAdaptor) GetChannelName() string { return "fake" }

// doOnceWithHeaders 通过真实的 DoRequestHelper 发一次请求，返回上游实际收到的请求头
func doOnceWithHeaders(t *testing.T, headers map[string]string) http.Header {
	t.Helper()

	var mu sync.Mutex
	var received http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client.Init()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	c.Request.Header.Set("Content-Type", "application/json")

	m := &meta.Meta{Config: model.ChannelConfig{Headers: headers}}
	a := &fakeAdaptor{url: srv.URL + "/v1/chat/completions"}
	resp, err := a.DoRequest(c, m, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("DoRequest 失败: %v", err)
	}
	defer resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	return received
}

func TestDoRequestHelperAppliesCustomHeaders(t *testing.T) {
	received := doOnceWithHeaders(t, map[string]string{
		"X-Title":       "my-gateway",
		"Authorization": "Custom overwritten",
	})

	if got := received.Get("X-Title"); got != "my-gateway" {
		t.Errorf("X-Title 未到达上游: got %q, want %q", got, "my-gateway")
	}
	if got := received.Get("Authorization"); got != "Custom overwritten" {
		t.Errorf("自定义头应覆盖适配器默认头: got %q, want %q", got, "Custom overwritten")
	}
	if got := received.Get("X-Adaptor-Only"); got != "yes" {
		t.Errorf("适配器自己的头不应被抹掉: got %q", got)
	}
	if got := received.Get("Content-Type"); got != "application/json" {
		t.Errorf("通用头不应被抹掉: got %q", got)
	}
}

// 回归守卫：没配自定义头的渠道（Headers 为 nil 或空 map）行为必须与改造前一致
func TestDoRequestHelperWithoutCustomHeadersIsUnchanged(t *testing.T) {
	cases := map[string]map[string]string{
		"nil 的 Headers":   nil,
		"空 map 的 Headers": {},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			received := doOnceWithHeaders(t, headers)

			if got := received.Get("Authorization"); got != "Bearer adaptor-default" {
				t.Errorf("无自定义头时适配器认证头应保持原样: got %q, want %q", got, "Bearer adaptor-default")
			}
			if got := received.Get("X-Adaptor-Only"); got != "yes" {
				t.Errorf("适配器自己的头不应被抹掉: got %q", got)
			}
			if got := received.Get("Content-Type"); got != "application/json" {
				t.Errorf("通用头不应被抹掉: got %q", got)
			}
			if got := received.Values("X-Title"); len(got) != 0 {
				t.Errorf("不应凭空多出自定义头: %v", got)
			}
		})
	}
}
