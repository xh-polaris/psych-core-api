package engine

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/alert"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"go.mongodb.org/mongo-driver/v2/bson"
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

// smsFilter 流式过滤：检测告警标记、发送 SMS、移除标记文本。
// 最小缓存策略：只缓存可能跨 chunk 截断的标记前缀，避免不必要的文字延迟。
type smsFilter struct {
	buf    strings.Builder
	marker string
	sent   bool
}

func (f *smsFilter) check(text string) (clean string, triggered bool) {
	combined := f.buf.String() + text
	f.buf.Reset()

	if !f.sent && strings.Contains(combined, f.marker) {
		triggered = true
		f.sent = true
	}

	clean = strings.ReplaceAll(combined, f.marker, "")

	keep := longestPrefix(clean, f.marker)
	if keep > 0 {
		if keep < len(clean) {
			f.buf.WriteString(clean[len(clean)-keep:])
			return clean[:len(clean)-keep], triggered
		}
		f.buf.WriteString(clean)
		return "", triggered
	}
	return clean, triggered
}

func (f *smsFilter) flush() string {
	s := f.buf.String()
	f.buf.Reset()
	return s
}

// longestPrefix 返回 s 中最长后缀的长度，该后缀是 marker 的前缀。
// 用于流式场景中判断最近若干字节是否可能构成跨 chunk 的标记片段。
func longestPrefix(s, marker string) int {
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(marker, s[i:]) {
			return len(s) - i
		}
	}
	return 0
}

// checkAlertSms 从 LLM 流中检测告警标记，首次命中时异步发送告警短信，并从流中移除标记。
func (e *Engine) checkAlertSms(ctx context.Context, input *schema.StreamReader[*schema.Message]) *schema.StreamReader[*schema.Message] {
	if input == nil {
		return nil
	}

	unitIdHex, _ := e.info[cst.JsonUnitID].(string)
	userIdHex, _ := e.info[cst.JsonUserID].(string)
	unitId, _ := bson.ObjectIDFromHex(unitIdHex)
	userId, _ := bson.ObjectIDFromHex(userIdHex)
	convId, _ := bson.ObjectIDFromHex(e.uSession)

	sf := &smsFilter{marker: cst.AlertSmsMarker}
	var once sync.Once

	output, writer := schema.Pipe[*schema.Message](5)

	go func() {
		defer writer.Close()
		defer input.Close()

		for {
			if ctx.Err() != nil {
				return
			}

			msg, err := input.Recv()
			if err != nil {
				if flushed := sf.flush(); flushed != "" {
					writer.Send(&schema.Message{Content: flushed}, nil)
				}
				writer.Send(nil, err)
				return
			}

			if msg != nil && msg.Content != "" {
				clean, triggered := sf.check(msg.Content)
				if triggered && !convId.IsZero() {
					once.Do(func() {
						logs.Infof("[engine] alert marker detected, dispatching alert for user %s", userId.Hex())
						e.llmWg.Add(1)
						go func() {
							defer e.llmWg.Done()
							bgCtx := context.WithoutCancel(ctx)
							sendCtx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
							defer cancel()
							if err := alert.Mgr.Send(sendCtx, unitId, userId, convId); err != nil {
								logs.Errorf("[engine] alert send err: %v", err)
							}
						}()
					})
				}
				msg.Content = clean
			}
			writer.Send(msg, nil)
		}
	}()

	return output
}
