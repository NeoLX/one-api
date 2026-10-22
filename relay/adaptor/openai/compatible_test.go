package openai

import (
	"testing"

	"github.com/songquanpeng/one-api/relay/channeltype"
)

// OpenCodeGo 的模型清单刻意留空（模型多且变动快，靠渠道页「从上游拉取模型」取实况）。
// 有值会让 /v1/models 与前端下拉显示错东西，所以锁住这个约定。
func TestGetCompatibleChannelMetaForOpenCodeGo(t *testing.T) {
	name, models := GetCompatibleChannelMeta(channeltype.OpenCodeGo)
	if name != "opencode-go" {
		t.Errorf("渠道名: got %q, want %q", name, "opencode-go")
	}
	if len(models) != 0 {
		t.Errorf("模型清单应为空，实际 %d 项：%v", len(models), models)
	}
	// nil 会被序列化成 JSON null（前端拿它当下拉 options 会崩），必须是空切片
	if models == nil {
		t.Error("应返回空切片而不是 nil")
	}
}
