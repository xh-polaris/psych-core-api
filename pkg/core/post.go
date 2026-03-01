package core

// 定义后处理相关内容

type (
	// PostNotify 提交给post服务的通知
	PostNotify struct {
		Session string         `json:"session"` // 对话标识
		UserId  string         `json:"userId"`  // 用户id
		UnitId  string         `json:"unitId"`  // 单位id
		Usage   *Usage         `json:"usage"`   // 用量
		Info    map[string]any `json:"info"`    // 额外信息
		Start   int64          `json:"start"`   // 对话开始时间戳(s)
		End     int64          `json:"end"`     // 对话结束时间戳(s)
		Config  *Config        `json:"config"`  // 对话配置
	}

	Usage struct {
		*LLMUsage
		*ASRUsage
		*TTSUsage
	}

	// LLMUsage is the token usage of the llm
	LLMUsage struct {
		// PromptTokens is the number of prompt tokens, including all the input tokens of this request.
		PromptTokens int `json:"promptTokens"`
		// PromptTokenDetails is a breakdown of the prompt tokens.
		PromptTokenDetails PromptTokenDetails `json:"promptCachedToken"`
		// CompletionTokens is the number of completion tokens.
		CompletionTokens int `json:"completionTokens"`
		// TotalTokens is the total number of tokens.
		TotalTokens int `json:"totalTokens"`
	}
	PromptTokenDetails struct {
		// Cached tokens present in the prompt.
		CachedTokens int `json:"cached_tokens"`
	}

	ASRUsage struct {
	}
	TTSUsage struct {
	}
)
