package impl

import "testing"

func TestChatCompletionsModelPath(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		wantPath   string
	}{
		{
			name:     "DeepSeek 使用默认路径",
			wantPath: "/chat/completions",
		},
		{
			name:       "兼容上游可覆盖请求路径",
			configured: "v1/chat/completions",
			wantPath:   "/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if path := normalizeChatCompletionsPath(tt.configured); path != tt.wantPath {
				t.Fatalf("请求路径 = %q，期望 %q", path, tt.wantPath)
			}
		})
	}
}
