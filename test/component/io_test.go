package component

import (
	"fmt"
	"github.com/xh-polaris/psych-pkg/core"
)

func Out(out *core.Channel[*core.Resp]) {
	for o := range out.C {
		fmt.Printf("[io test]ID: %d | Type: %d\nContent: %+v\n", o.ID, o.Type, o.Content)
		if o.Content == "end" {
			return
		}
	}
}
