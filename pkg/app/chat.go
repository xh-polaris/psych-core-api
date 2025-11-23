// Copyright © 2025 univero. All rights reserved.
// Licensed under the GNU Affero General Public License v3 (AGPL-3.0).
// license that can be found in the LICENSE file.

package app

import (
	"context"

	"github.com/cloudwego/eino/components/model"
)

type (
	// ChatApp 是对话大模型应用, 使用Eino的ToolCallingChatModel
	// 调用方通过sessionId来标识这一轮对话记录
	ChatApp = model.ToolCallingChatModel

	ChatSetting struct {
		Provider  string `json:"provider"`
		Url       string `json:"url"`
		Model     string `json:"model"`
		BotId     string `json:"botId"`
		UserId    string `json:"userId"`
		AccessKey string `json:"accessKey"`
	}

	// ChatFrame 一次响应
	ChatFrame struct {
		// Id 消息编号, 每次响应都应该从头统计
		Id uint64 `json:"id"`
		// Content 响应内容
		Content string `json:"content"`
		// SessionId 上下文标识
		SessionId string `json:"session_id"`
		// Timestamp 秒级时间戳
		Timestamp int64 `json:"timestamp"`
		// Finish 是否完成, stop正常完成, interrupt打断
		Finish string `json:"finish"`
	}
)

// chatFactory ChatApp的构造函数类型
type chatFactory func(ctx context.Context, uSession string, setting *ChatSetting) (ChatApp, error)

// chatProviders ChatApp的构造函数
var chatProviders = make(map[string]chatFactory)

// ChatRegister 注册一个ChatApp的构造函数
func ChatRegister(name string, factory chatFactory) {
	chatProviders[name] = factory
}

// NewChatApp 构造ChatApp的工厂方法
func NewChatApp(ctx context.Context, uSession string, setting *ChatSetting) (ChatApp, error) {
	if factory, ok := chatProviders[setting.Provider]; ok {
		return factory(ctx, uSession, setting)
	}
	return nil, NoFactory
}
