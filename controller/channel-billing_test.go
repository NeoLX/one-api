package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/songquanpeng/one-api/common/client"
	"github.com/songquanpeng/one-api/model"
)

func TestGetResponseBodyAppliesCustomHeaders(t *testing.T) {
	var received http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client.Init()

	channel := &model.Channel{
		Type:   1,
		Key:    "sk-test",
		Config: `{"headers":{"X-Title":"my-gateway"}}`,
	}
	body, err := GetResponseBody("GET", srv.URL, channel, GetAuthHeader(channel.Key))
	if err != nil {
		t.Fatalf("GetResponseBody 失败: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("返回体不对: %s", string(body))
	}
	if got := received.Get("X-Title"); got != "my-gateway" {
		t.Errorf("自定义头未到达上游: got %q, want %q", got, "my-gateway")
	}
	if got := received.Get("Authorization"); got != "Bearer sk-test" {
		t.Errorf("原有认证头不应被破坏: got %q", got)
	}
}

// 回归守卫：channel.config 是非法 JSON 时，余额查询必须照常工作（按零值配置继续）
func TestGetResponseBodyToleratesMalformedConfig(t *testing.T) {
	var received http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client.Init()

	channel := &model.Channel{
		Type:   1,
		Key:    "sk-test",
		Config: `{"headers": not-json`,
	}
	body, err := GetResponseBody("GET", srv.URL, channel, GetAuthHeader(channel.Key))
	if err != nil {
		t.Fatalf("配置非法时余额查询不应失败: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("返回体不对: %s", string(body))
	}
	if got := received.Get("Authorization"); got != "Bearer sk-test" {
		t.Errorf("原有认证头不应被破坏: got %q", got)
	}
}
