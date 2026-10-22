package adaptor

import (
	"net/http"
	"testing"
)

func TestApplyCustomHeaders(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]string
		custom   map[string]string
		expected map[string]string
	}{
		{
			name:     "空 map 不改变任何头",
			initial:  map[string]string{"Authorization": "Bearer old"},
			custom:   nil,
			expected: map[string]string{"Authorization": "Bearer old"},
		},
		{
			name:     "普通自定义头被写入",
			initial:  nil,
			custom:   map[string]string{"X-Title": "my-gateway"},
			expected: map[string]string{"X-Title": "my-gateway"},
		},
		{
			name:     "自定义头覆盖已存在的 Authorization",
			initial:  map[string]string{"Authorization": "Bearer old"},
			custom:   map[string]string{"Authorization": "Custom new"},
			expected: map[string]string{"Authorization": "Custom new"},
		},
		{
			name:     "黑名单头被跳过",
			initial:  map[string]string{"Host": "real-upstream"},
			custom:   map[string]string{"Host": "evil", "Content-Length": "1", "Connection": "close", "Accept-Encoding": "gzip"},
			expected: map[string]string{"Host": "real-upstream"},
		},
		{
			name:     "空键被跳过",
			initial:  nil,
			custom:   map[string]string{"": "v", "X-A": "1"},
			expected: map[string]string{"X-A": "1"},
		},
		{
			name:     "小写键名被 canonical 化并覆盖",
			initial:  map[string]string{"X-Title": "old"},
			custom:   map[string]string{"x-title": "new"},
			expected: map[string]string{"X-Title": "new"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range tt.initial {
				h.Set(k, v)
			}
			ApplyCustomHeaders(h, tt.custom)
			for k, v := range tt.expected {
				if got := h.Get(k); got != v {
					t.Errorf("header %s: got %q, want %q", k, got, v)
				}
			}
			if len(h) != len(tt.expected) {
				t.Errorf("header count: got %d (%v), want %d (%v)", len(h), h, len(tt.expected), tt.expected)
			}
		})
	}
}
