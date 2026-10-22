package channeltype

import (
	"testing"

	"github.com/songquanpeng/one-api/relay/apitype"
)

// ChannelBaseURLs 是靠下标与常量对齐的（url.go 的 init 只校验长度），
// 所以逐个断言关键类型的下标，防止以后往数组中间插项把大家错位。
func TestChannelBaseURLsAlignment(t *testing.T) {
	tests := []struct {
		channelType int
		want        string
	}{
		{OpenAI, "https://api.openai.com"},
		{SiliconFlow, "https://api.siliconflow.cn"},
		{GeminiOpenAICompatible, "https://generativelanguage.googleapis.com/v1beta/openai/"},
		{OpenCodeGo, "https://opencode.ai/inference/go/openai"},
	}
	for _, tt := range tests {
		if got := ChannelBaseURLs[tt.channelType]; got != tt.want {
			t.Errorf("ChannelBaseURLs[%d]: got %q, want %q", tt.channelType, got, tt.want)
		}
	}
}

// OpenCodeGo 复用 OpenAI 适配器，走默认的 {base}/v1/... 拼接
func TestToAPITypeForOpenCodeGo(t *testing.T) {
	if got := ToAPIType(OpenCodeGo); got != apitype.OpenAI {
		t.Errorf("ToAPIType(OpenCodeGo): got %d, want %d (OpenAI)", got, apitype.OpenAI)
	}
}
