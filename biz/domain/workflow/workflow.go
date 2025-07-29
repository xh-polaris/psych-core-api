package workflow

import (
	"context"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

// WorkFlow 工作流编排
type WorkFlow struct {
	// 引用engine, 通过engine来读写
	en    core.Engine
	close chan struct{}
	ctx   context.Context
	in    *core.Channel[*core.Cmd]

	chatApp   app.ChatApp
	ttsApp    app.TTSApp
	asrApp    app.ASRApp
	reportApp app.ReportApp

	history *HistoryPipe
	asr     *ASRPipe
	tts     *TTSPipe
	chat    *ChatPipe
	io      *IOPipe

	conf *core.WorkFlowConfig
}

// Orchestrate 编排工作流
// reportApp app不需要实例化, 保留配置即可
func (w *WorkFlow) Orchestrate(conf *core.WorkFlowConfig) (err error) {
	// 配置
	if err = w.config(conf); err != nil {
		return
	}
	// 编排
	out := core.NewChannel[*core.Resp](3, w.close)
	w.history = NewHistoryPipe(w.close)
	w.asr = NewASRPipe(w.ctx, w.close, w.asrApp)
	w.tts = NewTTSPipe(w.ctx, w.close, w.ttsApp, out)
	w.chat = NewChatPipe(w.ctx, w.close, w.chatApp, w.en.Session(), w.history.in, w.tts.in, out)
	w.io = NewIOPipe(w.close, w.in, w.asr.in, w.chat.in, w.history.in, out)
	return
}

// config 配置app
func (w *WorkFlow) config(conf *core.WorkFlowConfig) (err error) {
	w.conf = conf
	uSession := w.en.Session()

	if w.chatApp, err = app.NewChatApp(uSession, conf.ChatConfig); err != nil {
		logx.Error("[workflow] [config] new chatApp err: %v", err)
		return
	}
	if w.ttsApp, err = app.NewTTSApp(uSession, conf.TTSConfig); err != nil {
		logx.Error("[workflow] [config] new ttsApp err: %v", err)
		return
	}
	if w.asrApp, err = app.NewASRApp(uSession, conf.ASRConfig); err != nil {
		logx.Error("[workflow] [config] new asrApp err: %v", err)
		return
	}
	return
}

func (w *WorkFlow) WithIn(in *core.Channel[*core.Cmd]) core.WorkFlow {
	w.in = in
	return w
}

func (w *WorkFlow) WithContext(ctx context.Context) core.WorkFlow {
	w.ctx = ctx
	return w
}

func (w *WorkFlow) WithClose(close chan struct{}) core.WorkFlow {
	w.close = close
	return w
}
