package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-pkg/core"
)

var authType2Int32 = map[string]int32{
	"Already":             -1,
	"AuthStudentIdAndPwd": 1,
}

func main() {
	for start() {
	}
}
func start() bool {
	var (
		err         error
		ctx, cancel = context.WithCancel(context.Background())
		meta        *core.Meta
		conn        *websocket.Conn
	)
	// 连接WebSocket服务器
	if conn, meta, err = connectWebSocket(); err != nil {
		log.Println("连接失败:", err)
	}
	defer func() { _ = conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(time.Second)) }()
	// 启动心跳协程
	go heartbeat(ctx, conn)
	// 启动消息接收协程
	go receiveMessages(ctx, conn, meta)
	// 主协程处理用户输入
	return handleUserInput(cancel, conn, meta)
}

func connectWebSocket() (conn *websocket.Conn, meta *core.Meta, err error) {
	var message []byte
	if conn, _, err = websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/chat", nil); err != nil {
		return conn, meta, fmt.Errorf("连接服务器失败: %w", err)
	}
	// 读取元信息
	if _, message, err = conn.ReadMessage(); err != nil {
		return conn, meta, fmt.Errorf("读取元信息失败: %w", err)
	}
	meta = &core.Meta{}
	if err = json.Unmarshal(message, meta); err != nil {
		return conn, meta, fmt.Errorf("解析元信息失败: %w", err)
	}
	log.Printf("连接成功，协议版本: %d (序列化: %d, 压缩: %d)",
		meta.Version, meta.Serialization, meta.Compression)
	return conn, meta, nil
}

// 处理用户输入
func handleUserInput(cancel context.CancelFunc, conn *websocket.Conn, meta *core.Meta) (restart bool) {
	reader := bufio.NewReader(os.Stdin)
	for {
		printMenu()
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "auth":
			SendAuthMessage(conn, meta, reader)
		case "cmd":
			SendCommandMessage(conn, meta, reader)
		case "exit":
			cancel()
			return false
		case "restart":
			cancel()
			return true
		default:
			fmt.Println("无效输入，请重新选择")
		}
	}
}

// 菜单提示
func printMenu() {
	fmt.Println("\n请选择操作:")
	fmt.Println("auth. 发送认证消息")
	fmt.Println("cmd. 发送命令消息")
	fmt.Println("restart. 重启程序")
	fmt.Println("exit. 退出程序")
	fmt.Print("请输入选项: ")
}

// 根据提示词获取输入
func promptInput(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// 根据提示词输入音频
func promptInputAudio(reader *bufio.Reader, prompt string) []byte {
	fmt.Print(prompt)
	// TODO
	return nil
}
