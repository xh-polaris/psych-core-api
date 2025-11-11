package util

import (
	"errors"

	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-pkg/core"
)

func Convert[T any](in any) (out T, ok bool) {
	if v, ok := in.(T); ok {
		return v, true
	}
	return
}

func Err(err error) *core.Err {
	var custom errorx.StatusError
	if errors.As(err, &custom) {
		return &core.Err{Code: int(custom.Code()), Message: custom.Error()}
	}
	return &core.Err{Code: 999, Message: err.Error()}

}
