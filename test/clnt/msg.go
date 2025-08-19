package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-pkg/core"
	"log"
	"os"
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
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					log.Println("连接异常关闭:", err)
				}
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
			log.Printf("收到音频消息\n")
			modelVideo = append(modelVideo, []byte(payload.(*core.Resp).Content.(string))...)
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
