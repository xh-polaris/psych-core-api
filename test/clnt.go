package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-pkg/core"
)

var (
	conn          *websocket.Conn
	meta          *core.Meta
	heartbeatDone = make(chan struct{})
)

var authType2Int32 = map[string]int32{
	"Already":             -1,
	"AuthStudentIdAndPwd": 1,
}

func main() {
	// 连接WebSocket服务器
	if err := connectWebSocket(); err != nil {
		log.Fatal("连接失败:", err)
	}
	defer conn.Close()

	// 启动心跳协程
	go heartbeat()
	defer close(heartbeatDone)

	// 启动消息接收协程
	go receiveMessages()

	// 主协程处理用户输入
	handleUserInput()
}

func connectWebSocket() error {
	var err error
	conn, _, err = websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/chat", nil)
	if err != nil {
		return fmt.Errorf("连接服务器失败: %w", err)
	}

	// 读取元信息
	_, message, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取元信息失败: %w", err)
	}

	meta = &core.Meta{}
	if err := json.Unmarshal(message, meta); err != nil {
		return fmt.Errorf("解析元信息失败: %w", err)
	}

	log.Printf("连接成功，协议版本: %d (序列化: %d, 压缩: %d)",
		meta.Version, meta.Serialization, meta.Compression)
	return nil
}

func heartbeat() {
	log.Println("start heartbeat")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				log.Println("心跳发送失败:", err)
				return
			}
			//log.Println("发送心跳")
		case <-heartbeatDone:
			return
		}
	}
}

func receiveMessages() {
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Println("连接异常关闭:", err)
			}
			return
		}

		switch {
		case mt == websocket.PongMessage:
			log.Println("[心跳] 收到Pong响应")
		default:
			processBinaryMessage(data)
		}
	}
}

func processBinaryMessage(data []byte) {
	msg, err := core.MUnmarshal(data, meta.Compression, meta.Serialization)
	if err != nil {
		log.Println("消息解码失败:", err)
		return
	}

	payload, err := core.DecodeMessage(msg)
	if err != nil {
		log.Println("消息解析失败:", err)
		return
	}

	// 格式化输出
	jsonData, _ := json.MarshalIndent(payload, "", "  ")
	log.Printf("收到 %s 消息:\n%s\n", msg.Type, jsonData)
}

func handleUserInput() {
	reader := bufio.NewReader(os.Stdin)

	for {
		printMenu()
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			sendAuthMessage(reader)
		case "2":
			sendCommandMessage(reader)
		case "exit":
			return
		default:
			fmt.Println("无效输入，请重新选择")
		}
	}
}

func printMenu() {
	fmt.Println("\n请选择操作:")
	fmt.Println("1. 发送认证消息")
	fmt.Println("2. 发送命令消息")
	fmt.Println("exit. 退出程序")
	fmt.Print("请输入选项: ")
}

func sendAuthMessage(reader *bufio.Reader) {
	//auth := core.Auth{
	//	AuthType:   authType2Int32[promptInput(reader, "请输入AuthType: ")],
	//	AuthID:     promptInput(reader, "请输入AuthID: "),
	//	VerifyCode: promptInput(reader, "请输入VerifyCode: "),
	//	Info:       make(map[string]string),
	//}
	auth := core.Auth{
		AuthType:   authType2Int32["StudentIdAndPwd"],
		AuthID:     promptInput(reader, "请输入AuthID: "),
		VerifyCode: promptInput(reader, "请输入VerifyCode: "),
		Info:       make(map[string]string),
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

	// 发送消息
	if err := sendMessage(core.MAuth, &auth); err != nil {
		log.Println("发送认证消息失败:", err)
	} else {
		fmt.Println("认证消息发送成功")
	}
}

func sendCommandMessage(reader *bufio.Reader) {
	fmt.Println("\n请选择命令类型:")
	fmt.Println("1. 文字输入")
	fmt.Println("2. 音频输入")
	fmt.Println("3. 音频识别")
	choice := promptInput(reader, "请输入命令类型: ")

	var cmdType core.CType
	switch choice {
	case "1":
		cmdType = core.CUserText
	case "2":
		cmdType = core.CUserAudio
	case "3":
		cmdType = core.CUserAudioASR
	default:
		fmt.Println("无效的命令类型")
		return
	}

	content := promptInput(reader, "请输入命令内容: ")
	cmd := core.Cmd{Command: cmdType, Content: content}

	if err := sendMessage(core.MCmd, &cmd); err != nil {
		log.Println("发送命令失败:", err)
	}
}

func promptInput(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func sendMessage(mType core.MType, payload any) error {
	msg, err := core.EncodeMessage(mType, payload)
	if err != nil {
		return fmt.Errorf("编码消息失败: %w", err)
	}

	data, err := core.MMarshal(msg, meta.Compression, meta.Serialization)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}

	log.Printf("已发送 %s 消息", msg.Type)
	return nil
}
