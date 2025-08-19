package component

import (
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

type TestHistoryPipe struct {
	// unexpected func() 历史记录出错不会结束, 保证最低可用性
	session string
	in      *core.Channel[*core.HisEntry]
}

func NewTestHistoryPipe(close chan struct{}, session string) *TestHistoryPipe {
	return &TestHistoryPipe{
		session: session,
		in:      core.NewChannel[*core.HisEntry](3, close),
	}
}

// In 添加历史记录, 由in关闭
func (p *TestHistoryPipe) In() {
	for entry := range p.in.C {
		logx.Info("[his] add the history entry: %+v\n", entry)
	}
}

func (p *TestHistoryPipe) Run() {
	go p.In()
}

func (p *TestHistoryPipe) Close() {
	p.in.Close()
}
