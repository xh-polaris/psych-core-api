package engine

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
)

type bracketTracker struct {
	inBracket bool // 仅跟踪是否处于括号内
}

func (f *bracketTracker) filter(text string) (string, bool) {
	var b strings.Builder
	var hadBracket bool
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

// filterBracket removes any parenthesized content from the stream before downstream consumers.
func (e *Engine) filterBracket(ctx context.Context, input *schema.StreamReader[*schema.Message]) *schema.StreamReader[*schema.Message] {
	if input == nil {
		return nil
	}

	output, writer := schema.Pipe[*schema.Message](5)
	bt := &bracketTracker{}

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
				before := msg.Content
				after, hadBracket := bt.filter(before)
				if hadBracket {
					logs.Infof("[engine] bracket filtered: before=%q after=%q", before, after)
				}
				if after == "" && msg.ResponseMeta == nil && (hadBracket || bt.inBracket) {
					continue
				}
				msg.Content = after
			}
			writer.Send(msg, nil)
		}
	}()

	return output
}
