package engine

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// bracketFilter 流式过滤：按 rune 边界移除括号及括号内内容。
type bracketFilter struct {
	inBracket bool
}

func (f *bracketFilter) check(text string) (clean string, hadBracket bool) {
	var b strings.Builder
	for _, r := range text {
		switch r {
		case '(', '（':
			hadBracket = true
			if !f.inBracket {
				f.inBracket = true
			}
			continue
		case ')', '）':
			hadBracket = true
			if f.inBracket {
				f.inBracket = false
			}
			continue
		default:
			if !f.inBracket {
				b.WriteRune(r)
			}
		}
	}
	return b.String(), hadBracket
}

// checkBracket 从 LLM 流中移除括号内文本（如思考过程）。
func (e *Engine) checkBracket(ctx context.Context, input *schema.StreamReader[*schema.Message]) *schema.StreamReader[*schema.Message] {
	if input == nil {
		return nil
	}
	output, writer := schema.Pipe[*schema.Message](5)
	bf := &bracketFilter{}

	go func() {
		defer writer.Close()
		defer input.Close()
		for {
			if ctx.Err() != nil {
				return
			}
			msg, err := input.Recv()
			if err != nil {
				writer.Send(nil, err)
				return
			}
			if msg != nil && msg.Content != "" {
				after, hadBracket := bf.check(msg.Content)
				if after == "" && msg.ResponseMeta == nil && (hadBracket || bf.inBracket) {
					continue
				}
				msg.Content = after
			}
			writer.Send(msg, nil)
		}
	}()
	return output
}
