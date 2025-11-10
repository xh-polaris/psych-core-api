package workflow

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/redis"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

type HistoryPipe struct {
	// unexpected func() 历史记录出错不会结束, 保证最低可用性
	session string
	rs      *core.HisRedis
	in      *core.Channel[*core.HisEntry]
}

func NewHistoryPipe(close chan struct{}, session string) *HistoryPipe {
	return &HistoryPipe{
		session: session,
		rs:      core.GetHisRedis(redis.NewRedis(conf.GetConfig())),
		in:      core.NewChannel[*core.HisEntry](3, close),
	}
}

// In 添加历史记录, 由in关闭
func (p *HistoryPipe) In() {
	var err error
	for entry := range p.in.C {
		utils.DPrint("[his] add the history entry: %+v\n", entry) //debug
		if err = p.rs.Add(p.session, entry); err != nil {
			logx.Error("[history pipe] add err:%v", err)
		}
	}
}

func (p *HistoryPipe) Run() {
	go p.In()
}

func (p *HistoryPipe) Close() {
	p.in.Close()
}
