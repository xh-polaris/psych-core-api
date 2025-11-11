package util

import (
	"fmt"
)

func DPrint(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
