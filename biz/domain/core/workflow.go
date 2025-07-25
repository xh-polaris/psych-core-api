package core

import (
	"github.com/xh-polaris/psych-pkg/biz/domain/engine"
)

// WorkFlow 工作流编排
type WorkFlow struct {
	// 引擎的指针, 因为会用到一部分的字段
	*engine.Engine
}

// Run 启动工作流
func (wf *WorkFlow) Run() (err error) {
	return
}
