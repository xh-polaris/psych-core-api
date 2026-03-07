package httpx

import "github.com/xh-polaris/psych-core-api/biz/application/dto/basic"

func succeed() *basic.Response {
	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}
}
