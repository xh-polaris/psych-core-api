package main

import (
	"bufio"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-pkg/core"
	"log"
)

func SendCommandMessage(conn *websocket.Conn, meta *core.Meta, reader *bufio.Reader) {
	fmt.Println("\n请选择命令类型:")
	fmt.Println("1. 文字输入")
	fmt.Println("2. 音频输入")
	fmt.Println("3. 音频识别")
	choice := promptInput(reader, "请输入命令类型: ")

	var cmdType core.CType
	var content any
	switch choice {
	case "1":
		cmdType = core.CUserText
		content = promptInput(reader, "请输入命令内容: ")
	case "2":
		cmdType = core.CUserAudio
		content = promptInputAudio(reader, "请输入音频")
	case "3":
		cmdType = core.CUserAudioASR
	default:
		fmt.Println("无效的命令类型")
		return
	}

	cmd := core.Cmd{Command: cmdType, Content: content}

	if err := sendMessage(conn, meta, core.MCmd, &cmd); err != nil {
		log.Println("发送命令失败:", err)
	}
}
