package main

import (
	"context"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

func heartbeat(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				log.Println("心跳发送失败:", err)
				return
			}
			ticker.Reset(time.Second)
			//log.Println("heartbeat")
		}
	}
}
