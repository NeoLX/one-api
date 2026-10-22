package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUserProxyTestDB(t *testing.T) {
	t.Helper()
	// RedisEnabled 默认为 true，只有启动时 InitRedisClient 才会置 false；
	// 单测里必须显式关掉，否则 CacheGetUserGroup/CacheGetUserProxyConfig 会走 Redis 分支
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = originalRedisEnabled })

	var err error
	model.DB, err = gorm.Open(sqlite.Open("file::userproxy?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err = model.DB.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	for _, table := range []string{"users", "channels", "abilities"} {
		if err := model.DB.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("清空 %s 失败: %v", table, err)
		}
	}
}

func createProxyUser(t *testing.T, role int) *model.User {
	t.Helper()
	user := &model.User{
		Username: fmt.Sprintf("proxy-user-%d-%d", role, len(t.Name())),
		Password: "password",
		Role:     role,
		Status:   model.UserStatusEnabled,
		Group:    "default",
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	return user
}

func createProxyChannel(t *testing.T, channelType int, models string) {
	t.Helper()
	priority := int64(0)
	channel := &model.Channel{
		Type:     channelType,
		Key:      "sk-test",
		Name:     "proxy-channel",
		Status:   model.ChannelStatusEnabled,
		Models:   models,
		Group:    "default",
		Priority: &priority,
	}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatalf("插入渠道失败: %v", err)
	}
	if err := channel.AddAbilities(); err != nil {
		t.Fatalf("写入 abilities 失败: %v", err)
	}
}

func newUserProxyContext(t *testing.T, userId int, method string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/user/proxy", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.Id, userId)
	return c, w
}

type userProxyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Config     map[string][]string `json:"config"`
		Options    []userProxyOption   `json:"options"`
		Restricted bool                `json:"restricted"`
	} `json:"data"`
}

func decodeUserProxyResponse(t *testing.T, w *httptest.ResponseRecorder) userProxyResponse {
	t.Helper()
	var resp userProxyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	return resp
}

// 可选项只来自该用户分组内的启用渠道，且带供应商显示名
func TestGetUserProxyOptions(t *testing.T) {
	setupUserProxyTestDB(t)
	user := createProxyUser(t, model.RoleCommonUser)
	createProxyChannel(t, channeltype.OpenAI, "gpt-3.5-turbo,gpt-4o")
	createProxyChannel(t, channeltype.OpenCodeGo, "glm-5.2")

	c, w := newUserProxyContext(t, user.Id, http.MethodGet, "")
	GetUserProxy(c)
	resp := decodeUserProxyResponse(t, w)
	if !resp.Success {
		t.Fatalf("期望成功，实际 message=%s", resp.Message)
	}
	if resp.Data.Restricted {
		t.Error("未配置时不应是受限状态")
	}
	if len(resp.Data.Config) != 0 {
		t.Errorf("未配置时 config 应为空，实际 %v", resp.Data.Config)
	}
	if len(resp.Data.Options) != 2 {
		t.Fatalf("可选项应有 2 个供应商，实际 %d：%+v", len(resp.Data.Options), resp.Data.Options)
	}
	if resp.Data.Options[0].Type != channeltype.OpenAI || strings.Join(resp.Data.Options[0].Models, ",") != "gpt-3.5-turbo,gpt-4o" {
		t.Errorf("第一个可选项不对：%+v", resp.Data.Options[0])
	}
	if resp.Data.Options[0].Name == "" || resp.Data.Options[1].Name == "" {
		t.Errorf("供应商显示名不应为空：%+v", resp.Data.Options)
	}
}

// 分组外的渠道不能出现在可选项里
func TestGetUserProxyOptionsExcludesOtherGroups(t *testing.T) {
	setupUserProxyTestDB(t)
	user := createProxyUser(t, model.RoleCommonUser)
	createProxyChannel(t, channeltype.OpenAI, "gpt-3.5-turbo")
	otherGroupChannel := &model.Channel{
		Type: channeltype.DeepSeek, Key: "sk-test", Name: "vip-only",
		Status: model.ChannelStatusEnabled, Models: "deepseek-chat", Group: "vip",
	}
	if err := model.DB.Create(otherGroupChannel).Error; err != nil {
		t.Fatalf("插入 vip 渠道失败: %v", err)
	}

	c, w := newUserProxyContext(t, user.Id, http.MethodGet, "")
	GetUserProxy(c)
	resp := decodeUserProxyResponse(t, w)
	for _, option := range resp.Data.Options {
		if option.Type == channeltype.DeepSeek {
			t.Fatalf("分组外的供应商不应出现在可选项里：%+v", option)
		}
	}
}

func TestUpdateUserProxySavesAndReadsBack(t *testing.T) {
	setupUserProxyTestDB(t)
	user := createProxyUser(t, model.RoleCommonUser)
	createProxyChannel(t, channeltype.OpenAI, "gpt-3.5-turbo,gpt-4o")

	c, w := newUserProxyContext(t, user.Id, http.MethodPut, `{"config":{"1":["gpt-4o"]}}`)
	UpdateUserProxy(c)
	if resp := decodeUserProxyResponse(t, w); !resp.Success {
		t.Fatalf("保存失败：%s", resp.Message)
	}

	raw, err := model.GetUserProxyConfig(user.Id)
	if err != nil {
		t.Fatalf("读库失败: %v", err)
	}
	if raw != `{"1":["gpt-4o"]}` {
		t.Errorf("落库内容不对: %q", raw)
	}

	// 回读：生效规则可解析，且受限
	rules, err := model.CacheGetUserProxyRules(user.Id)
	if err != nil {
		t.Fatalf("读规则失败: %v", err)
	}
	if rules == nil || len(rules[channeltype.OpenAI]) != 1 || rules[channeltype.OpenAI][0] != "gpt-4o" {
		t.Errorf("规则不对: %#v", rules)
	}
}

// 空配置 = 关闭限制
func TestUpdateUserProxyEmptyConfigMeansUnrestricted(t *testing.T) {
	setupUserProxyTestDB(t)
	user := createProxyUser(t, model.RoleCommonUser)
	createProxyChannel(t, channeltype.OpenAI, "gpt-3.5-turbo")

	c, w := newUserProxyContext(t, user.Id, http.MethodPut, `{"config":{}}`)
	UpdateUserProxy(c)
	if resp := decodeUserProxyResponse(t, w); !resp.Success {
		t.Fatalf("保存空配置失败：%s", resp.Message)
	}
	raw, _ := model.GetUserProxyConfig(user.Id)
	if raw != "" {
		t.Errorf("空配置应落库为空串，实际 %q", raw)
	}
	c2, w2 := newUserProxyContext(t, user.Id, http.MethodGet, "")
	GetUserProxy(c2)
	if resp := decodeUserProxyResponse(t, w2); resp.Data.Restricted {
		t.Error("空配置应为不受限")
	}
}

// 只允许收窄：提交分组外的供应商、或该供应商下没有的模型，都必须 400
func TestUpdateUserProxyRejectsEscalation(t *testing.T) {
	setupUserProxyTestDB(t)
	user := createProxyUser(t, model.RoleCommonUser)
	createProxyChannel(t, channeltype.OpenAI, "gpt-3.5-turbo")

	tests := []struct {
		name     string
		body     string
		wantWord string
	}{
		{name: "分组外供应商", body: `{"config":{"52":[]}}`, wantWord: "供应商不可用"},
		{name: "该供应商下没有的模型", body: `{"config":{"1":["gpt-4o"]}}`, wantWord: "模型不可用"},
		{name: "供应商 id 非法", body: `{"config":{"abc":[]}}`, wantWord: "非法"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := newUserProxyContext(t, user.Id, http.MethodPut, tt.body)
			UpdateUserProxy(c)
			resp := decodeUserProxyResponse(t, w)
			if resp.Success {
				t.Fatalf("越权提交应当失败：%s", tt.body)
			}
			if !strings.Contains(resp.Message, tt.wantWord) {
				t.Errorf("错误文案应含 %q，实际 %q", tt.wantWord, resp.Message)
			}
			raw, _ := model.GetUserProxyConfig(user.Id)
			if raw != "" {
				t.Errorf("被拒的提交不应落库，实际 %q", raw)
			}
		})
	}
}

// root 恒定不受限：配了也不生效
func TestUserProxyDoesNotRestrictRoot(t *testing.T) {
	setupUserProxyTestDB(t)
	root := createProxyUser(t, model.RoleRootUser)
	createProxyChannel(t, channeltype.OpenAI, "gpt-3.5-turbo")

	c, w := newUserProxyContext(t, root.Id, http.MethodPut, `{"config":{"1":["gpt-3.5-turbo"]}}`)
	UpdateUserProxy(c)
	if resp := decodeUserProxyResponse(t, w); !resp.Success {
		t.Fatalf("root 保存失败：%s", resp.Message)
	}
	rules, err := model.CacheGetUserProxyRules(root.Id)
	if err != nil {
		t.Fatalf("读规则失败: %v", err)
	}
	if rules != nil {
		t.Errorf("root 的规则应为 nil（不受限），实际 %#v", rules)
	}
	c2, w2 := newUserProxyContext(t, root.Id, http.MethodGet, "")
	GetUserProxy(c2)
	if resp := decodeUserProxyResponse(t, w2); resp.Data.Restricted {
		t.Error("root 不应显示为受限")
	}
}

// 可用模型 = 分组模型 ∩ 代理白名单
func TestGetUserAvailableModelsIntersectsProxyRules(t *testing.T) {
	setupUserProxyTestDB(t)
	user := createProxyUser(t, model.RoleCommonUser)
	createProxyChannel(t, channeltype.OpenAI, "model-a,model-b")

	readModels := func(t *testing.T) []string {
		t.Helper()
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/user/available_models", nil)
		c.Set(ctxkey.Id, user.Id)
		GetUserAvailableModels(c)
		var resp struct {
			Success bool     `json:"success"`
			Message string   `json:"message"`
			Data    []string `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
		}
		if !resp.Success {
			t.Fatalf("期望成功：%s", resp.Message)
		}
		return resp.Data
	}

	if got := strings.Join(readModels(t), ","); got != "model-a,model-b" {
		t.Errorf("未配置代理时应返回分组模型: got %q", got)
	}

	if err := model.UpdateUserProxyConfig(user.Id, `{"1":["model-a"]}`); err != nil {
		t.Fatalf("写入代理配置失败: %v", err)
	}
	if got := strings.Join(readModels(t), ","); got != "model-a" {
		t.Errorf("配置代理后应只剩被允许的模型: got %q", got)
	}
}

// 「空数组=该供应商全部模型」在展示上必须展开成该供应商实际提供的模型，
// 不能放行分组里所有模型（否则会展示出用户其实调不通的模型）
func TestGetUserAvailableModelsEmptyListMeansProviderModelsOnly(t *testing.T) {
	setupUserProxyTestDB(t)
	user := createProxyUser(t, model.RoleCommonUser)
	createProxyChannel(t, channeltype.OpenAI, "model-a,model-b")
	createProxyChannel(t, channeltype.Custom, "model-c")

	if err := model.UpdateUserProxyConfig(user.Id, `{"8":[]}`); err != nil {
		t.Fatalf("写入代理配置失败: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/available_models", nil)
	c.Set(ctxkey.Id, user.Id)
	GetUserAvailableModels(c)

	var resp struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("期望成功：%s", resp.Message)
	}
	if strings.Join(resp.Data, ",") != "model-c" {
		t.Errorf("只允许 Custom 全部模型时应只展示 model-c: got %v", resp.Data)
	}
}
