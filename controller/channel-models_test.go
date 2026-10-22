package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/client"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestParseOpenAIStyleModels(t *testing.T) {
	models, err := parseOpenAIStyleModels([]byte(`{"data":[{"id":"b-model"},{"id":"a-model"},{"id":""}]}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := strings.Join(models, ","); got != "a-model,b-model" {
		t.Errorf("got %q, want %q", got, "a-model,b-model")
	}
}

func TestParseOpenAIStyleModelsBadJSON(t *testing.T) {
	if _, err := parseOpenAIStyleModels([]byte(`not json`)); err == nil {
		t.Errorf("非法 JSON 应当报错")
	}
}

func TestParseGeminiModels(t *testing.T) {
	models, err := parseGeminiModels([]byte(`{"models":[{"name":"models/gemini-1.5-pro"},{"name":"models/gemini-1.5-flash"}]}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := strings.Join(models, ","); got != "gemini-1.5-flash,gemini-1.5-pro" {
		t.Errorf("got %q, want %q（models/ 前缀应被去掉）", got, "gemini-1.5-flash,gemini-1.5-pro")
	}
}

func TestParseOllamaModels(t *testing.T) {
	models, err := parseOllamaModels([]byte(`{"models":[{"name":"llama3:8b"},{"name":"qwen2:7b"}]}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := strings.Join(models, ","); got != "llama3:8b,qwen2:7b" {
		t.Errorf("got %q, want %q", got, "llama3:8b,qwen2:7b")
	}
}

func TestSupportsFetchModels(t *testing.T) {
	supported := []int{
		channeltype.OpenAI, channeltype.Custom, channeltype.OpenAICompatible,
		channeltype.GeminiOpenAICompatible, channeltype.DeepSeek, channeltype.OpenCodeGo,
		channeltype.Azure, channeltype.Anthropic, channeltype.Gemini, channeltype.Ollama,
	}
	for _, ct := range supported {
		if !supportsFetchModels(ct) {
			t.Errorf("渠道类型 %d 应支持自动拉取模型", ct)
		}
	}
	unsupported := []int{
		channeltype.AwsClaude, channeltype.Baidu, channeltype.Proxy, channeltype.VertextAI,
		channeltype.Minimax, channeltype.Doubao, channeltype.Novita, channeltype.BaiduV2, channeltype.AliBailian,
	}
	for _, ct := range unsupported {
		if supportsFetchModels(ct) {
			t.Errorf("渠道类型 %d 不应支持自动拉取模型", ct)
		}
	}
	// 边界用哨兵 Dummy 表示，别再写死具体数字（新增渠道类型会让旧数字变成合法类型）
	for _, ct := range []int{-1, channeltype.Dummy, channeltype.Dummy + 100} {
		if supportsFetchModels(ct) {
			t.Errorf("越界类型 %d 不应支持自动拉取模型", ct)
		}
	}
}

func TestBuildModelsRequest(t *testing.T) {
	base := "https://api.example.com"
	tests := []struct {
		name                string
		channelType         int
		config              string
		wantURL             string
		wantHeader          map[string]string
		requireNoAuthHeader bool
	}{
		{
			name:        "OpenAI 兼容默认走 /v1/models",
			channelType: channeltype.DeepSeek,
			wantURL:     base + "/v1/models",
			wantHeader:  map[string]string{"Authorization": "Bearer sk-test"},
		},
		{
			name:        "Azure 用 api-key 与 api-version",
			channelType: channeltype.Azure,
			config:      `{"api_version":"2024-10-01-preview"}`,
			wantURL:     base + "/openai/models?api-version=2024-10-01-preview",
			wantHeader:  map[string]string{"api-key": "sk-test"},
		},
		{
			name:        "Anthropic 用 x-api-key",
			channelType: channeltype.Anthropic,
			wantURL:     base + "/v1/models",
			wantHeader:  map[string]string{"x-api-key": "sk-test", "anthropic-version": "2023-06-01"},
		},
		{
			name:                "Gemini 认证走 query 参数",
			channelType:         channeltype.Gemini,
			wantURL:             base + "/v1beta/models?key=sk-test&pageSize=1000",
			wantHeader:          map[string]string{},
			requireNoAuthHeader: true,
		},
		{
			name:                "Ollama 走 /api/tags 且不认证",
			channelType:         channeltype.Ollama,
			wantURL:             base + "/api/tags",
			wantHeader:          map[string]string{},
			requireNoAuthHeader: true,
		},
		{
			name:        "自定义头会一起带上",
			channelType: channeltype.OpenAI,
			config:      `{"headers":{"X-Title":"my-gateway"}}`,
			wantURL:     base + "/v1/models",
			wantHeader:  map[string]string{"Authorization": "Bearer sk-test", "X-Title": "my-gateway"},
		},
		{
			name:                "OpenCodeGo 模型列表在 /openai 的上一层，且不带认证",
			channelType:         channeltype.OpenCodeGo,
			wantURL:             base + "/v1/models",
			wantHeader:          map[string]string{},
			requireNoAuthHeader: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL := base
			ch := &model.Channel{
				Type:    tt.channelType,
				Key:     "sk-test",
				BaseURL: &baseURL,
				Config:  tt.config,
			}
			req, err := buildModelsRequest(ch)
			if err != nil {
				t.Fatalf("构造请求失败: %v", err)
			}
			if req.URL.String() != tt.wantURL {
				t.Errorf("URL: got %q, want %q", req.URL.String(), tt.wantURL)
			}
			for k, v := range tt.wantHeader {
				if got := req.Header.Get(k); got != v {
					t.Errorf("header %s: got %q, want %q", k, got, v)
				}
			}
			if tt.requireNoAuthHeader {
				if got := req.Header.Get("Authorization"); got != "" {
					t.Errorf("该类型不应设置 Authorization，实际 %q", got)
				}
				if got := req.Header.Get("api-key"); got != "" {
					t.Errorf("该类型不应设置 api-key，实际 %q", got)
				}
			}
		})
	}
}

// OpenCodeGo 的聊天端点带 /openai 前缀而模型列表不带，所以拉取时要把它去掉；
// base 不以 /openai 结尾时退回 {base}/v1/models；没填 base_url 时用类型内置地址
func TestBuildModelsRequestOpenCodeGo(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantURL string
	}{
		{
			name:    "没填 base_url 用内置地址",
			baseURL: "",
			wantURL: "https://opencode.ai/inference/go/v1/models",
		},
		{
			name:    "自建镜像同样剥掉 /openai",
			baseURL: "https://mirror.example.com/go/openai",
			wantURL: "https://mirror.example.com/go/v1/models",
		},
		{
			name:    "base 不以 /openai 结尾时退回 /v1/models",
			baseURL: "https://mirror.example.com/go",
			wantURL: "https://mirror.example.com/go/v1/models",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &model.Channel{Type: channeltype.OpenCodeGo, Key: "sk-test"}
			if tt.baseURL != "" {
				baseURL := tt.baseURL
				ch.BaseURL = &baseURL
			}
			req, err := buildModelsRequest(ch)
			if err != nil {
				t.Fatalf("构造请求失败: %v", err)
			}
			if req.URL.String() != tt.wantURL {
				t.Errorf("URL: got %q, want %q", req.URL.String(), tt.wantURL)
			}
			if got := req.Header.Get("Authorization"); got != "" {
				t.Errorf("模型列表端点是公开的，不应带认证头，实际 %q", got)
			}
		})
	}
}

// 类型 50/51 的 base URL 约定与中继一致：base URL 自带 /v1（或 /v1beta/openai），路径里不能再拼 /v1
func TestBuildModelsRequestForBaseURLIncludesV1Types(t *testing.T) {
	cases := []struct {
		name        string
		channelType int
		baseURL     string
		wantURL     string
		wantErr     bool
	}{
		{
			name:        "OpenAICompatible 用户填的 base_url 自带 /v1",
			channelType: channeltype.OpenAICompatible,
			baseURL:     "https://api.example.com/v1",
			wantURL:     "https://api.example.com/v1/models",
		},
		{
			name:        "GeminiOpenAICompatible 走默认 base URL",
			channelType: channeltype.GeminiOpenAICompatible,
			wantURL:     "https://generativelanguage.googleapis.com/v1beta/openai/models",
		},
		{
			name:        "该类型没有默认地址又没填 base_url 时报错",
			channelType: channeltype.Custom,
			wantErr:     true,
		},
		{
			name:        "越界类型不应 panic 而应报错",
			channelType: 999,
			wantErr:     true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ch := &model.Channel{Type: tt.channelType, Key: "sk-test"}
			if tt.baseURL != "" {
				ch.BaseURL = &tt.baseURL
			}
			req, err := buildModelsRequest(ch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际拿到 URL %q", req.URL.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("构造请求失败: %v", err)
			}
			if req.URL.String() != tt.wantURL {
				t.Errorf("URL: got %q, want %q", req.URL.String(), tt.wantURL)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	short := "abc"
	if got := truncateString(short, 500); got != short {
		t.Errorf("短字符串不应被改动: got %q", got)
	}

	long := strings.Repeat("a", 600)
	got := truncateString(long, 500)
	if utf8.RuneCountInString(got) != 503 {
		t.Errorf("期望 500 个字符 + \"...\"，实际 %d", utf8.RuneCountInString(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("截断后应有省略号: %q", got)
	}

	chinese := strings.Repeat("中", 600)
	chineseGot := truncateString(chinese, 500)
	if !utf8.ValidString(chineseGot) {
		t.Errorf("截断后必须是合法 UTF-8: %q", chineseGot)
	}
	if utf8.RuneCountInString(chineseGot) != 503 {
		t.Errorf("中文也应按字符数截断，实际 %d", utf8.RuneCountInString(chineseGot))
	}
}

func newFetchModelsContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/fetch_models", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestFetchChannelModelsUnsupportedType(t *testing.T) {
	c, w := newFetchModelsContext(t, `{"type":33,"key":"sk-test"}`)
	FetchChannelModels(c)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Errorf("不支持的渠道类型不应返回成功")
	}
	if !strings.Contains(resp.Message, "暂不支持自动拉取模型") {
		t.Errorf("提示文案不对: %q", resp.Message)
	}
}

func TestFetchChannelModelsKeyRequired(t *testing.T) {
	c, w := newFetchModelsContext(t, `{"type":1,"key":""}`)
	FetchChannelModels(c)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Errorf("没有密钥也没有 id 时不应返回成功")
	}
	if !strings.Contains(resp.Message, "请先填写密钥") {
		t.Errorf("提示文案不对: %q", resp.Message)
	}
}

func TestFetchChannelModelsSuccess(t *testing.T) {
	var received http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"fake-gpt-4"},{"id":"fake-gpt-3.5"},{"id":"fake-gpt-4"}]}`))
	}))
	defer srv.Close()

	client.Init()

	body := fmt.Sprintf(`{"type":1,"key":"sk-fake","base_url":%q,"config":"{\"headers\":{\"X-Title\":\"my-gateway\"}}"}`, srv.URL)
	c, w := newFetchModelsContext(t, body)
	FetchChannelModels(c)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Models []string `json:"models"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v，body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("期望成功，实际 body=%s", w.Body.String())
	}
	if got := strings.Join(resp.Data.Models, ","); got != "fake-gpt-3.5,fake-gpt-4" {
		t.Errorf("模型列表不对（应去重排序）: got %q, want %q", got, "fake-gpt-3.5,fake-gpt-4")
	}
	if got := received.Get("X-Title"); got != "my-gateway" {
		t.Errorf("自定义头未到达上游: got %q", got)
	}
	if got := received.Get("Authorization"); got != "Bearer sk-fake" {
		t.Errorf("认证头不对: got %q", got)
	}
}

func TestFetchChannelModelsUpstream401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Authentication Fails"}}`))
	}))
	defer srv.Close()

	client.Init()

	body := fmt.Sprintf(`{"type":1,"key":"sk-bad","base_url":%q}`, srv.URL)
	c, w := newFetchModelsContext(t, body)
	FetchChannelModels(c)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Errorf("上游 401 不应返回成功")
	}
	if !strings.Contains(resp.Message, "401") || !strings.Contains(resp.Message, "Authentication Fails") {
		t.Errorf("错误文案应带上游状态码与响应片段: %q", resp.Message)
	}
}

// 上游请求失败时，错误文本里不能出现渠道密钥（Gemini 的 key 在 URL query 里）
func TestFetchModelsErrorDoesNotLeakKey(t *testing.T) {
	client.Init()

	const secret = "sk-super-secret-key"

	t.Run("网络失败", func(t *testing.T) {
		baseURL := "http://127.0.0.1:1"
		ch := &model.Channel{
			Type:    channeltype.Gemini,
			Key:     secret,
			BaseURL: &baseURL,
		}
		_, err := fetchModelsFromUpstream(ch)
		if err == nil {
			t.Fatalf("期望请求失败，实际成功")
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("错误文本泄露了密钥: %q", err.Error())
		}
		if !strings.Contains(err.Error(), "请求上游失败") {
			t.Errorf("错误文本应说明是上游请求失败: %q", err.Error())
		}
	})

	t.Run("地址非法", func(t *testing.T) {
		baseURL := "http://exa mple.com"
		ch := &model.Channel{
			Type:    channeltype.Gemini,
			Key:     secret,
			BaseURL: &baseURL,
		}
		_, err := fetchModelsFromUpstream(ch)
		if err == nil {
			t.Fatalf("期望构造请求失败，实际成功")
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("错误文本泄露了密钥: %q", err.Error())
		}
	})
}

func setupFetchModelsTestDB(t *testing.T) {
	t.Helper()
	var err error
	model.DB, err = gorm.Open(sqlite.Open("file::fetchmodels?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err = model.DB.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if err := model.DB.Exec("DELETE FROM channels").Error; err != nil {
		t.Fatalf("清空 channels 表失败: %v", err)
	}
}

func fetchModelsResponse(t *testing.T, w *httptest.ResponseRecorder) (bool, string, []string) {
	t.Helper()
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Models []string `json:"models"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	return resp.Success, resp.Message, resp.Data.Models
}

// 编辑态只传 id 时，必须沿用库里存的渠道类型，否则地址拼接与"是否支持"判断都会走错分支
func TestFetchChannelModelsByIdUsesStoredType(t *testing.T) {
	setupFetchModelsTestDB(t)
	client.Init()

	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-1.5-pro"}]}`))
	}))
	defer srv.Close()

	baseURL := srv.URL
	ch := &model.Channel{
		Type:    channeltype.Gemini,
		Key:     "sk-gemini-secret",
		Name:    "gemini-by-id",
		Status:  model.ChannelStatusEnabled,
		BaseURL: &baseURL,
	}
	if err := model.DB.Create(ch).Error; err != nil {
		t.Fatalf("插入渠道失败: %v", err)
	}

	c, w := newFetchModelsContext(t, fmt.Sprintf(`{"id":%d}`, ch.Id))
	FetchChannelModels(c)

	ok, message, models := fetchModelsResponse(t, w)
	if !ok {
		t.Fatalf("期望成功，实际 message=%s", message)
	}
	if gotPath != "/v1beta/models" {
		t.Errorf("应按库里的 Gemini 类型拼地址: got path %q", gotPath)
	}
	if !strings.Contains(gotQuery, "key=sk-gemini-secret") {
		t.Errorf("Gemini 认证应走 query 参数: got %q", gotQuery)
	}
	if got := strings.Join(models, ","); got != "gemini-1.5-pro" {
		t.Errorf("模型名应去掉 models/ 前缀: got %q", got)
	}
}

// 编辑态只传 id 时，库里是不支持的类型要给"暂不支持"，而不是地址类错误
func TestFetchChannelModelsByIdUsesStoredUnsupportedType(t *testing.T) {
	setupFetchModelsTestDB(t)
	client.Init()

	baseURL := "http://127.0.0.1:9101"
	ch := &model.Channel{
		Type:    channeltype.AwsClaude,
		Key:     "sk-aws",
		Name:    "aws-by-id",
		Status:  model.ChannelStatusEnabled,
		BaseURL: &baseURL,
	}
	if err := model.DB.Create(ch).Error; err != nil {
		t.Fatalf("插入渠道失败: %v", err)
	}

	c, w := newFetchModelsContext(t, fmt.Sprintf(`{"id":%d}`, ch.Id))
	FetchChannelModels(c)

	ok, message, _ := fetchModelsResponse(t, w)
	if ok {
		t.Errorf("不支持的渠道类型不应返回成功")
	}
	if !strings.Contains(message, "暂不支持自动拉取模型") {
		t.Errorf("提示文案不对: %q", message)
	}
}

// Ollama 不需要密钥，未填密钥也应允许拉取模型
func TestFetchChannelModelsOllamaNeedsNoKey(t *testing.T) {
	setupFetchModelsTestDB(t)
	client.Init()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:8b"}]}`))
	}))
	defer srv.Close()

	baseURL := srv.URL
	ch := &model.Channel{
		Type:    channeltype.Ollama,
		Name:    "ollama-by-id",
		Status:  model.ChannelStatusEnabled,
		BaseURL: &baseURL,
	}
	if err := model.DB.Create(ch).Error; err != nil {
		t.Fatalf("插入渠道失败: %v", err)
	}

	c, w := newFetchModelsContext(t, fmt.Sprintf(`{"id":%d}`, ch.Id))
	FetchChannelModels(c)

	ok, message, models := fetchModelsResponse(t, w)
	if !ok {
		t.Fatalf("Ollama 无密钥也应能拉取: message=%s", message)
	}
	if got := strings.Join(models, ","); got != "llama3:8b" {
		t.Errorf("模型列表不对: got %q", got)
	}
}
