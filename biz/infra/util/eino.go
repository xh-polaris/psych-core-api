package util

import "github.com/cloudwego/eino/schema"

func AddExtra(m *schema.Message, key string, value any) {
	if m.Extra == nil {
		m.Extra = make(map[string]any)
	}
	m.Extra[key] = value
}
