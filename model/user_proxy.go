package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/logger"
)

// 用户级「代理」白名单：用户自助配置「我能用哪些供应商的哪些模型」，
// 只做减法（可选项来自其分组内本来就能用的渠道），未配置等于不限制。

// ParseUserProxyConfig 解析用户代理白名单，返回 nil 表示不限制：
// 空串、null、{}、非法 JSON（记 syslog 但不拦人）都视为不限制。
// 键=渠道类型 id（供应商）；值=允许的模型名；值为空数组=该供应商全部模型。
func ParseUserProxyConfig(raw string) map[int][]string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "{}" {
		return nil
	}
	parsed := make(map[string][]string)
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		logger.SysError("解析用户代理配置失败，按不限制处理: " + err.Error())
		return nil
	}
	if len(parsed) == 0 {
		return nil
	}
	rules := make(map[int][]string, len(parsed))
	for key, models := range parsed {
		channelType, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil {
			logger.SysError("用户代理配置里的供应商 id 非法，已忽略: " + key)
			continue
		}
		rules[channelType] = models
	}
	if len(rules) == 0 {
		return nil
	}
	return rules
}

// IsModelAllowedByProxy 判断某模型是否被代理白名单允许；rules 为 nil 表示不限制。
func IsModelAllowedByProxy(rules map[int][]string, modelName string) bool {
	if rules == nil || modelName == "" {
		return true
	}
	for _, models := range rules {
		if len(models) == 0 {
			return true // 该供应商全部模型
		}
		for _, m := range models {
			if m == modelName {
				return true
			}
		}
	}
	return false
}

// AllowedChannelTypesForModel 返回针对某模型放行的渠道类型集合；nil 表示不限制。
// 同名模型挂在多个供应商上时，用这个集合把渠道选择收窄到被允许的供应商。
func AllowedChannelTypesForModel(rules map[int][]string, modelName string) map[int]bool {
	if rules == nil {
		return nil
	}
	allowed := make(map[int]bool, len(rules))
	for channelType, models := range rules {
		if len(models) == 0 {
			allowed[channelType] = true
			continue
		}
		for _, m := range models {
			if m == modelName {
				allowed[channelType] = true
				break
			}
		}
	}
	return allowed
}

func GetUserProxyConfig(id int) (string, error) {
	var user User
	err := DB.Select("proxy_config").First(&user, id).Error
	if err != nil {
		return "", err
	}
	return user.ProxyConfig, nil
}

// CacheGetUserProxyRules 返回该用户生效的代理白名单；未配置或 root 用户返回 nil（不限制）。
// root 恒定不受限，避免管理员把自己锁死在门外。
func CacheGetUserProxyRules(id int) (map[int][]string, error) {
	raw, err := CacheGetUserProxyConfig(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	// 只有配了限制的人才需要多查一次角色（root 例外）
	var user User
	if err = DB.Select("role").First(&user, id).Error; err != nil {
		return nil, err
	}
	if user.Role == RoleRootUser {
		return nil, nil
	}
	return ParseUserProxyConfig(raw), nil
}

// AllowedModelsForUser 算出白名单允许的模型名集合；rules 为 nil 表示不限制（返回 nil）。
// 与 IsModelAllowedByProxy 的区别：这里知道"某个供应商在分组里到底提供哪些模型"，
// 所以"空数组=该供应商全部模型"会展开成该供应商实际提供的模型，而不是放行所有模型。
// 用于给用户展示可用模型清单，避免展示出他其实用不了的模型。
func AllowedModelsForUser(rules map[int][]string, providerModels map[int][]string) map[string]bool {
	if rules == nil {
		return nil
	}
	allowed := make(map[string]bool)
	for channelType, models := range rules {
		if len(models) == 0 {
			for _, m := range providerModels[channelType] {
				allowed[m] = true
			}
			continue
		}
		for _, m := range models {
			allowed[m] = true
		}
	}
	return allowed
}

// UpdateUserProxyConfig 保存用户的代理白名单原始 JSON，并清掉缓存
func UpdateUserProxyConfig(id int, raw string) error {
	if err := DB.Model(&User{}).Where("id = ?", id).Update("proxy_config", raw).Error; err != nil {
		return err
	}
	if common.RedisEnabled {
		if err := common.RedisDel(fmt.Sprintf("user_proxy_config:%d", id)); err != nil {
			logger.SysError("Redis del user proxy config error: " + err.Error())
		}
	}
	return nil
}

// GetGroupProviderModels 汇总某分组下启用渠道提供的 供应商（渠道类型）→ 模型名。
// 这是用户自助可选项的唯一来源，保证自助只能收窄、不能放宽。
func GetGroupProviderModels(group string) (map[int][]string, error) {
	var channels []*Channel
	if err := DB.Where("status = ?", ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	result := make(map[int][]string)
	seen := make(map[int]map[string]bool)
	for _, channel := range channels {
		inGroup := false
		for _, g := range strings.Split(channel.Group, ",") {
			if strings.TrimSpace(g) == group {
				inGroup = true
				break
			}
		}
		if !inGroup {
			continue
		}
		if seen[channel.Type] == nil {
			seen[channel.Type] = make(map[string]bool)
		}
		for _, m := range strings.Split(channel.Models, ",") {
			m = strings.TrimSpace(m)
			if m == "" || seen[channel.Type][m] {
				continue
			}
			seen[channel.Type][m] = true
			result[channel.Type] = append(result[channel.Type], m)
		}
	}
	for channelType := range result {
		sort.Strings(result[channelType])
	}
	return result, nil
}
