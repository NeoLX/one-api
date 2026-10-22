package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/i18n"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay"
	"github.com/songquanpeng/one-api/relay/channeltype"
	"github.com/songquanpeng/one-api/relay/meta"
)

type userProxyOption struct {
	Type   int      `json:"type"`
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

type userProxyUpdateRequest struct {
	Config map[string][]string `json:"config"`
}

func getChannelTypeName(channelType int) string {
	adaptor := relay.GetAdaptor(channeltype.ToAPIType(channelType))
	adaptor.Init(&meta.Meta{ChannelType: channelType})
	return adaptor.GetChannelName()
}

// buildUserProxyOptions 汇总该用户分组下可用的 供应商 → 模型，作为自助可选项。
// 可选项只来自分组内本就能用的渠道，所以自助配置只能收窄。
func buildUserProxyOptions(group string) ([]userProxyOption, error) {
	providerModels, err := model.GetGroupProviderModels(group)
	if err != nil {
		return nil, err
	}
	options := make([]userProxyOption, 0, len(providerModels))
	for channelType, models := range providerModels {
		options = append(options, userProxyOption{
			Type:   channelType,
			Name:   getChannelTypeName(channelType),
			Models: models,
		})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Type < options[j].Type })
	return options, nil
}

func stringInSlice(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// GetUserProxy 返回当前用户生效的代理白名单与其可选范围
func GetUserProxy(c *gin.Context) {
	userId := c.GetInt(ctxkey.Id)
	userGroup, err := model.CacheGetUserGroup(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	options, err := buildUserProxyOptions(userGroup)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	rules, err := model.CacheGetUserProxyRules(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// 只回传"当前实际生效"的规则：root 的配置不生效，这里就是空的
	config := make(map[string][]string, len(rules))
	for channelType, models := range rules {
		if models == nil {
			models = []string{}
		}
		config[strconv.Itoa(channelType)] = models
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"config":     config,
			"options":    options,
			"restricted": rules != nil,
		},
	})
	return
}

// UpdateUserProxy 保存当前用户的代理白名单。服务端二次校验，只允许收窄：
// 提交的供应商必须在其分组可用范围内，模型必须属于该供应商。
func UpdateUserProxy(c *gin.Context) {
	userId := c.GetInt(ctxkey.Id)
	var req userProxyUpdateRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.Translate(c, "invalid_parameter")})
		return
	}
	userGroup, err := model.CacheGetUserGroup(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	options, err := buildUserProxyOptions(userGroup)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	available := make(map[int][]string, len(options))
	for _, option := range options {
		available[option.Type] = option.Models
	}

	cleaned := make(map[string][]string)
	for typeKey, models := range req.Config {
		channelType, err := strconv.Atoi(typeKey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("供应商 id 非法：%s", typeKey)})
			return
		}
		availableModels, ok := available[channelType]
		if !ok {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("供应商不可用：%d", channelType)})
			return
		}
		if len(models) == 0 {
			cleaned[typeKey] = []string{} // 该供应商全部模型
			continue
		}
		deduped := make([]string, 0, len(models))
		for _, m := range models {
			if !stringInSlice(availableModels, m) {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("模型不可用：%s", m)})
				return
			}
			if !stringInSlice(deduped, m) {
				deduped = append(deduped, m)
			}
		}
		cleaned[typeKey] = deduped
	}

	raw := ""
	if len(cleaned) > 0 {
		encoded, err := json.Marshal(cleaned)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		raw = string(encoded)
	}
	if err := model.UpdateUserProxyConfig(userId, raw); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
	return
}
