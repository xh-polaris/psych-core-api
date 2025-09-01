package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
)

var audioPath = "D:\\Projects\\xhpolaris\\psych\\psych-core-api\\test\\output.pcm"
var first, last = []byte{app.FirstASR}, []byte{app.LastASR}

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
		cmd := core.Cmd{Command: cmdType, Content: content}

		file, err := os.Open(audioPath)
		if err != nil {
			log.Fatalf("无法打开音频文件: %v", err)
		}
		buf := make([]byte, 3200) // 每次发送3200字节（约200ms 16kHz音频）
		cmd.Content = first       // 首包
		if err = sendMessage(conn, meta, core.MCmd, &cmd); err != nil {
			log.Println("发送命令失败:", err)
		}
		for {
			n, err := file.Read(buf)
			if err == io.EOF {
				log.Println("音频发送完成")
				break
			}
			if err != nil {
				log.Printf("读取音频失败: %v", err)
				break
			}
			cmd.Content = buf[:n]
			if err = sendMessage(conn, meta, core.MCmd, &cmd); err != nil {
				log.Println("发送命令失败:", err)
			}
		}
		cmd.Content = last // 尾包
		if err = sendMessage(conn, meta, core.MCmd, &cmd); err != nil {
			log.Println("发送命令失败:", err)
		}
		return
	default:
		fmt.Println("无效的命令类型")
		return
	}

	cmd := core.Cmd{Command: cmdType, Content: content}

	if err := sendMessage(conn, meta, core.MCmd, &cmd); err != nil {
		log.Println("发送命令失败:", err)
	}
}
