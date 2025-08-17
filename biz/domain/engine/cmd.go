package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/domain/workflow"
	"github.com/xh-polaris/psych-pkg/core"
)

func buildCmd(e *Engine) {
	e.cmdCh = core.NewChannel[*core.Cmd](3, e.close)
	e.workflow = &workflow.WorkFlow{}
	e.workflow.WithEngine(e).WithIn(e.cmdCh).WithContext(e.ctx).WithClose(e.close) // 配置workflow的输入流
}
