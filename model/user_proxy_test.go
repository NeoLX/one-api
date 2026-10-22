package model

import (
	"reflect"
	"testing"
)

func TestParseUserProxyConfig(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[int][]string
	}{
		{name: "空串=不限制", raw: "", want: nil},
		{name: "纯空白=不限制", raw: "   ", want: nil},
		{name: "null=不限制", raw: "null", want: nil},
		{name: "空对象=不限制", raw: "{}", want: nil},
		{name: "非法 JSON=不限制", raw: "{不是 json", want: nil},
		{name: "非对象 JSON=不限制", raw: `["52"]`, want: nil},
		{name: "键不是数字=忽略后为空=不限制", raw: `{"abc":["m"]}`, want: nil},
		{
			name: "空数组=该供应商全部模型",
			raw:  `{"52":[]}`,
			want: map[int][]string{52: {}},
		},
		{
			name: "正常配置",
			raw:  `{"52":["glm-5.2"],"1":[]}`,
			want: map[int][]string{52: {"glm-5.2"}, 1: {}},
		},
		{
			name: "混合非法键时保留合法键",
			raw:  `{"52":["a"],"x":["b"]}`,
			want: map[int][]string{52: {"a"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseUserProxyConfig(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseUserProxyConfig(%q): got %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestIsModelAllowedByProxy(t *testing.T) {
	tests := []struct {
		name      string
		rules     map[int][]string
		modelName string
		want      bool
	}{
		{name: "未配置=不限制", rules: nil, modelName: "anything", want: true},
		{name: "模型名为空=不限制", rules: map[int][]string{52: {"a"}}, modelName: "", want: true},
		{name: "供应商全开", rules: map[int][]string{52: {}}, modelName: "任意模型", want: true},
		{name: "命中清单", rules: map[int][]string{52: {"glm-5.2"}}, modelName: "glm-5.2", want: true},
		{name: "不在清单", rules: map[int][]string{52: {"glm-5.2"}}, modelName: "kimi-k3", want: false},
		{
			name:      "多个供应商里命中其一",
			rules:     map[int][]string{52: {"a"}, 1: {}},
			modelName: "b",
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsModelAllowedByProxy(tt.rules, tt.modelName); got != tt.want {
				t.Errorf("IsModelAllowedByProxy: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllowedChannelTypesForModel(t *testing.T) {
	tests := []struct {
		name      string
		rules     map[int][]string
		modelName string
		want      map[int]bool
	}{
		{name: "未配置=不限制", rules: nil, modelName: "a", want: nil},
		{name: "供应商全开", rules: map[int][]string{52: {}}, modelName: "a", want: map[int]bool{52: true}},
		{name: "命中清单", rules: map[int][]string{52: {"a"}}, modelName: "a", want: map[int]bool{52: true}},
		{name: "不在清单=空集合", rules: map[int][]string{52: {"a"}}, modelName: "b", want: map[int]bool{}},
		{
			name:      "同名模型只放行配置了它的供应商",
			rules:     map[int][]string{52: {"dup"}, 1: {}},
			modelName: "dup",
			want:      map[int]bool{52: true, 1: true},
		},
		{
			name:      "另一供应商清单里没有该模型则不含它",
			rules:     map[int][]string{52: {"dup"}, 8: {"other"}},
			modelName: "dup",
			want:      map[int]bool{52: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AllowedChannelTypesForModel(tt.rules, tt.modelName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AllowedChannelTypesForModel: got %#v, want %#v", got, tt.want)
			}
		})
	}
}
