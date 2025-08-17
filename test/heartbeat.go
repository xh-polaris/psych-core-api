package main

import (
	"github.com/gorilla/websocket"
	"log"
	"time"
)

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
