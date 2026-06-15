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

type smsAlertTracker struct {
	builder strings.Builder
	once    sync.Once
	marker  string
	ctx     context.Context
	wg      *sync.WaitGroup
	unitId  bson.ObjectID
	userId  bson.ObjectID
	convId  bson.ObjectID
}

func (t *smsAlertTracker) check(text string) string {
	t.builder.WriteString(text)
	content := t.builder.String()
	if strings.Contains(content, t.marker) && !t.convId.IsZero() {
		t.once.Do(func() {
			logs.Infof("[engine] alert marker detected, dispatching alert for user %s", t.userId.Hex())
			t.wg.Add(1)
			go func() {
				defer t.wg.Done()
				bgCtx := context.WithoutCancel(t.ctx)
				sendCtx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
				defer cancel()
				if err := alert.Mgr.Send(sendCtx, t.unitId, t.userId, t.convId); err != nil {
					logs.Errorf("[engine] alert send err: %v", err)
				}
			}()
		})
	}
	return strings.ReplaceAll(text, t.marker, "")
}

func (e *Engine) filterAlertSms(ctx context.Context, input *schema.StreamReader[*schema.Message]) *schema.StreamReader[*schema.Message] {
	if input == nil {
		return nil
	}

	unitIdHex, _ := e.info[cst.JsonUnitID].(string)
	userIdHex, _ := e.info[cst.JsonUserID].(string)
	unitId, _ := bson.ObjectIDFromHex(unitIdHex)
	userId, _ := bson.ObjectIDFromHex(userIdHex)
	convId, _ := bson.ObjectIDFromHex(e.uSession)

	tracker := &smsAlertTracker{
		marker: cst.AlertSmsMarker,
		ctx:    ctx,
		wg:     &e.llmWg,
		unitId: unitId,
		userId: userId,
		convId: convId,
	}

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
				writer.Send(nil, err)
				return
			}

			if msg != nil && msg.Content != "" {
				msg.Content = tracker.check(msg.Content)
			}
			writer.Send(msg, nil)
		}
	}()

	return output
}
