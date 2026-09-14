package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/core"
)

var modelVideo []byte

// 接受消息
func receiveMessages(ctx context.Context, conn *websocket.Conn, meta *core.Meta) {
	for {
		select {
		case <-ctx.Done():
			outputFile, err := os.Create("./output.pcm")
			if err != nil {
				log.Println("文件创建失败")
				return
			}
			if _, err = outputFile.Write(modelVideo); err != nil {
				log.Printf("音频写入失败:%s", err)
			}
			return
		default:
			mt, data, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-ctx.Done(): // 主动退出/重启, 不再提示
					return
				default:
				}
				// 服务端在空闲(默认 10 分钟无对话活动)后会主动结束会话并关闭连接
				log.Printf("连接已断开（服务端空闲后会自动结束会话）: %v", err)
				log.Println("如需继续，请输入 restart 重新建立连接（会创建新的 conversationId）")
				return
			}
			switch {
			case mt == websocket.PongMessage:
				log.Println("[心跳] 收到Pong响应")
			default:
				processBinaryMessage(data, meta)
			}
		}
	}
}

// 解码消息并格式化输出
func processBinaryMessage(data []byte, meta *core.Meta) {
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
	switch msg.Type {
	case core.MResp: // 响应消息
		switch payload.(*core.Resp).Type {
		case core.RModelAudio: // 模型音频
			if !ttsConnected { // 仅打印一条连接成功
				ttsConnected = true
				log.Printf("[tts] 音频通道已连接, 开始接收语音")
			}
			if *verbose { // 详细模式才打印每条音频
				log.Printf("收到音频消息, length=%d", len(payload.(*core.Resp).Content.(string)))
			}
			if payload.(*core.Resp) != nil && payload.(*core.Resp).Content != nil {
				modelVideo = append(modelVideo, []byte(payload.(*core.Resp).Content.(string))...)
			}
		case core.RModelText: // 模型文本, 累积到完整回复
			// Content 经 JSON 反序列化后为 map[string]any, 需重新序列化还原 ChatFrame
			raw, err := json.Marshal(payload.(*core.Resp).Content)
			if err != nil {
				log.Println("模型文本帧序列化失败:", err)
				break
			}
			var frame app.ChatFrame
			if err = json.Unmarshal(raw, &frame); err != nil {
				log.Println("模型文本帧解析失败:", err)
				break
			}
			modelText.WriteString(frame.Content)
			if *verbose { // 详细模式打印每帧
				log.Printf("收到文本帧 id=%d: %s", frame.Id, frame.Content)
			}
			if frame.Finish == "stop" { // 完整回复结束
				log.Printf("===== 模型完整回复 =====\n%s\n======================", modelText.String())
				modelText.Reset()
			}
		default:
			// 格式化输出
			jsonData, _ := json.MarshalIndent(payload, "", "  ")
			log.Printf("收到 %d 消息:\n%s\n", msg.Type, jsonData)
		}
	default:
		// 格式化输出
		jsonData, _ := json.MarshalIndent(payload, "", "  ")
		log.Printf("收到 %d 消息:\n%s\n", msg.Type, jsonData)
	}

}

var ttsConnected bool
var modelText strings.Builder // 累积模型文本回复

// 发送消息
func sendMessage(conn *websocket.Conn, meta *core.Meta, mType core.MType, payload any) error {
	msg, err := core.EncodeMessage(mType, payload)
	if err != nil {
		return fmt.Errorf("编码消息失败: %w", err)
	}
	data, err := core.MMarshal(msg, meta.Compression, meta.Serialization)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}
	if err = conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}
	fmt.Printf("[clnt] send message type %d\n %+v\n", mType, payload)
	return nil
}
