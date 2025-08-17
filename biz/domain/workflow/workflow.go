package workflow

import (
	"context"
	"github.com/xh-polaris/psych-pkg/app"
	_ "github.com/xh-polaris/psych-pkg/app/bailian"
	_ "github.com/xh-polaris/psych-pkg/app/volc"
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
	// optimize 应该允许没有tts或asr或report
	chatApp   app.ChatApp
	ttsApp    app.TTSApp
	asrApp    app.ASRApp
	reportApp app.ReportApp

	broadcast []core.Pipe
	history   *HistoryPipe
	asr       *ASRPipe
	tts       *TTSPipe
	chat      *ChatPipe
	io        *IOPipe

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
	out := core.NewChannel[*core.Resp](5, w.close)
	w.history = NewHistoryPipe(w.close, w.en.Session())
	w.asr = NewASRPipe(w.ctx, w.UnExpected, w.close, w.asrApp, out)
	w.tts = NewTTSPipe(w.ctx, w.UnExpected, w.close, w.ttsApp, out)
	w.chat = NewChatPipe(w.ctx, w.UnExpected, w.close, w.chatApp, w.en.Session(), w.history.in, w.tts.in, out)
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
	if w.asrApp, err = app.NewASRApp(uSession, conf.ASRConfig); err != nil {
		logx.Error("[workflow] [config] new asrApp err: %v", err)
		return
	}
	if w.ttsApp, err = app.NewTTSApp(uSession, conf.TTSConfig); err != nil {
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

func (w *WorkFlow) WithEngine(e core.Engine) core.WorkFlow {
	w.en = e
	return w
}

// Close 关闭workflow, 释放资源
func (w *WorkFlow) Close() (err error) {
	// 当engine close后, workflow中的ch大部分都会自动关闭, 为了避免泄露, 再次手动关闭
	for _, pipe := range w.broadcast {
		pipe.Close()
	}
	return
}

// UnExpected 因错误结束
func (w *WorkFlow) UnExpected() {
	w.en.Write(core.EndErr)
	logx.Info("[engine] close by workflow error")
	_ = w.en.Close()
}
