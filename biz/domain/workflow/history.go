package workflow

import (
	"fmt"
	"github.com/xh-polaris/psych-pkg/core"
	"time"
)

// HisEntry 历史记录条目
type HisEntry struct {
	Role      string
	Content   string
	Timestamp time.Time
}

type HistoryPipe struct {
	in *core.Channel[*HisEntry]
}

func NewHistoryPipe(close chan struct{}) *HistoryPipe {
	return &HistoryPipe{
		in: core.NewChannel[*HisEntry](3, close),
	}
}

func (p *HistoryPipe) In() {
	for entry := range p.in.C {
		// TODO 存储历史记录
		fmt.Println(entry)
	}
}

func (p *HistoryPipe) Run() {
	go p.In()
}
