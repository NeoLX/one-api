package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/client"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/apitype"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

const modelsResponseBodyLimit = 500

// supportsFetchModels 判断该渠道类型是否支持调用上游拉取模型列表
func supportsFetchModels(channelType int) bool {
	switch channelType {
	case channeltype.Azure, channeltype.Anthropic, channeltype.Gemini, channeltype.Ollama:
		return true
	case channeltype.Minimax, channeltype.Doubao, channeltype.Novita, channeltype.BaiduV2, channeltype.AliBailian:
		// 这些类型的中继路径不是 {base}/v1/...（见 relay/adaptor/openai/adaptor.go 的 GetRequestURL 分派），
		// 无法用统一规则推导模型列表地址，v1 不支持自动拉取
		return false
	}
	if channelType < 0 || channelType >= len(channeltype.ChannelBaseURLs) {
		return false
	}
	// OpenAI 兼容系（Custom / OpenAICompatible / GeminiOpenAICompatible / DeepSeek / Groq / Moonshot / XAI / SiliconFlow / OpenRouter 等）
	return channeltype.ToAPIType(channelType) == apitype.OpenAI
}

// buildModelsRequest 按渠道类型构造拉取模型列表的上游请求
func buildModelsRequest(channel *model.Channel) (*http.Request, error) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("解析渠道配置失败: %v", err)
	}
	baseURL := strings.TrimSuffix(channel.GetBaseURL(), "/")
	if baseURL == "" && channel.Type >= 0 && channel.Type < len(channeltype.ChannelBaseURLs) {
		baseURL = strings.TrimSuffix(channeltype.ChannelBaseURLs[channel.Type], "/")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("该渠道未填写上游地址，无法拉取模型")
	}

	var requestURL string
	switch channel.Type {
	case channeltype.Azure:
		apiVersion := cfg.APIVersion
		if apiVersion == "" {
			apiVersion = "2024-03-01-preview"
		}
		requestURL = fmt.Sprintf("%s/openai/models?api-version=%s", baseURL, apiVersion)
	case channeltype.Gemini:
		requestURL = fmt.Sprintf("%s/v1beta/models?key=%s&pageSize=1000", baseURL, channel.Key)
	case channeltype.Ollama:
		requestURL = fmt.Sprintf("%s/api/tags", baseURL)
	case channeltype.GeminiOpenAICompatible:
		// 该类型 base URL 自带 /v1beta/openai，中继不拼 /v1
		// （见 relay/adaptor/geminiv2/main.go GetRequestURL）
		requestURL = fmt.Sprintf("%s/models", baseURL)
	case channeltype.OpenCodeGo:
		// 聊天端点是 {base}/v1/chat/completions，但模型列表在上一层：
		// {base 去掉 /openai}/v1/models。base 不以 /openai 结尾（如自建镜像）时退回 {base}/v1/models
		requestURL = fmt.Sprintf("%s/v1/models", strings.TrimSuffix(baseURL, "/openai"))
	default:
		// 复用中继自身的拼接逻辑：OpenAICompatible 这类 base URL 自带 /v1 的类型会剥掉路径的 /v1
		requestURL = openai.GetFullRequestURL(baseURL, "/v1/models", channel.Type)
	}

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造上游请求失败: %s", redactKey(err.Error(), channel.Key))
	}
	switch channel.Type {
	case channeltype.Azure:
		req.Header.Set("api-key", channel.Key)
	case channeltype.Anthropic:
		req.Header.Set("x-api-key", channel.Key)
		req.Header.Set("anthropic-version", "2023-06-01")
	case channeltype.Gemini, channeltype.Ollama:
		// Gemini 认证走 query 参数，Ollama 无需认证
	case channeltype.OpenCodeGo:
		// 模型列表端点是公开的，且带上无效凭证反而会被上游拒（401），所以不发认证头
	default:
		req.Header.Set("Authorization", "Bearer "+channel.Key)
	}
	adaptor.ApplyCustomHeaders(req.Header, cfg.Headers)
	return req, nil
}

// fetchModelsFromUpstream 调上游模型列表接口并返回去重排序后的模型名
func fetchModelsFromUpstream(channel *model.Channel) ([]string, error) {
	if !supportsFetchModels(channel.Type) {
		return nil, fmt.Errorf("该供应商暂不支持自动拉取模型，请手动填写")
	}
	req, err := buildModelsRequest(channel)
	if err != nil {
		return nil, err
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求上游失败: %s", redactKey(err.Error(), channel.Key))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取上游响应失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("上游返回 %d: %s", resp.StatusCode, truncateString(string(body), modelsResponseBodyLimit))
	}
	switch channel.Type {
	case channeltype.Gemini:
		return parseGeminiModels(body)
	case channeltype.Ollama:
		return parseOllamaModels(body)
	default:
		return parseOpenAIStyleModels(body)
	}
}

// parseOpenAIStyleModels 解析 OpenAI 兼容的 {"data":[{"id":...}]}
func parseOpenAIStyleModels(body []byte) ([]string, error) {
	var resp struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(resp.Data))
	for _, m := range resp.Data {
		models = append(models, m.Id)
	}
	return dedupeSortedModels(models), nil
}

// parseGeminiModels 解析 {"models":[{"name":"models/gemini-1.5-pro"}]} 并去掉 models/ 前缀
func parseGeminiModels(body []byte) ([]string, error) {
	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(resp.Models))
	for _, m := range resp.Models {
		models = append(models, strings.TrimPrefix(m.Name, "models/"))
	}
	return dedupeSortedModels(models), nil
}

// parseOllamaModels 解析 {"models":[{"name":"llama3:8b"}]}
func parseOllamaModels(body []byte) ([]string, error) {
	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(resp.Models))
	for _, m := range resp.Models {
		models = append(models, m.Name)
	}
	return dedupeSortedModels(models), nil
}

func dedupeSortedModels(models []string) []string {
	seen := make(map[string]bool, len(models))
	out := make([]string, 0, len(models))
	for _, m := range models {
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

func truncateString(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// redactKey 抹掉错误文本里可能出现的渠道密钥
// （Gemini 类型的 key 在 URL query 里，网络错误的 *url.Error 会带出完整 URL）
func redactKey(s string, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "***")
}

type fetchModelsRequest struct {
	Id      int    `json:"id"`
	Type    int    `json:"type"`
	BaseURL string `json:"base_url"`
	Key     string `json:"key"`
	Config  string `json:"config"`
}

// FetchChannelModels 让管理员在保存渠道前/后都能拉取上游模型列表
func FetchChannelModels(c *gin.Context) {
	var req fetchModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channel := &model.Channel{Type: req.Type, Key: req.Key, Config: req.Config}
	if req.BaseURL != "" {
		channel.BaseURL = &req.BaseURL
	}
	// 编辑态前端只传 id（GET /api/channel/:id 不回传 key），请求里缺的字段一律以库里的为准，
	// 尤其 type：它决定上游地址怎么拼、以及该供应商是否支持拉取
	if req.Id != 0 {
		stored, err := model.GetChannelById(req.Id, true)
		if err != nil {
			if channel.Key == "" {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道不存在"})
				return
			}
		} else {
			if channel.Type == 0 {
				channel.Type = stored.Type
			}
			if channel.Key == "" {
				channel.Key = stored.Key
			}
			if channel.Config == "" {
				channel.Config = stored.Config
			}
			if channel.BaseURL == nil {
				channel.BaseURL = stored.BaseURL
			}
		}
	}
	// Ollama 无需认证，没有密钥也应能拉取
	if channel.Key == "" && channel.Type != channeltype.Ollama {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请先填写密钥"})
		return
	}
	models, err := fetchModelsFromUpstream(channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"models": models}})
}
