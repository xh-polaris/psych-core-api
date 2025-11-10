package main

import (
	"bufio"
	"fmt"
	"log"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-core-api/biz/infra/consts"
	"github.com/xh-polaris/psych-pkg/core"
)

var customUser = false

func SendAuthMessage(conn *websocket.Conn, meta *core.Meta, reader *bufio.Reader) {
	var auth core.Auth
	if !customUser {
		auth = core.Auth{
			AuthType:   authType2Int32["AuthStudentIdAndPwd"],
			AuthID:     "hsdsfz2025",                                              //promptInput(reader, "请输入AuthID: "),
			VerifyCode: "123456",                                                  //promptInput(reader, "请输入VerifyCode: "),
			Info:       map[string]any{consts.UnitId: "683beddbdcc71f894d67e3b3"}, //make(map[string]any),
		}
	} else {
		auth = core.Auth{
			AuthID:     promptInput(reader, "请输入用户ID"),
			AuthType:   authType2Int32[promptInput(reader, "请输入认证类型")],
			VerifyCode: promptInput(reader, "请输入凭证"),
			Info:       make(map[string]any),
		}
		// 交互式收集Info字段
		fmt.Println("\n请输入Info键值对（输入格式：key value，单独输入done结束）:")
		for {
			input := promptInput(reader, "info> ")
			if input == "done" {
				break
			}

			parts := strings.SplitN(input, " ", 2)
			if len(parts) != 2 {
				fmt.Println("输入格式错误，请按 key value 格式输入")
				continue
			}

			auth.Info[parts[0]] = parts[1]
			fmt.Printf("已添加: %s = %s\n", parts[0], parts[1])
		}
	}
	// 发送消息
	if err := sendMessage(conn, meta, core.MAuth, &auth); err != nil {
		log.Println("发送认证消息失败:", err)
	} else {
		fmt.Println("认证消息发送成功")
	}
}
