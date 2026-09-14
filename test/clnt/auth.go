package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/core"
)

var customUser = false

var (
	authToken      string // HTTP 登录获取的 JWT
	conversationId string // 启动时自动创建的会话 ID
)

// prepareSession 启动时通过 HTTP 登录并创建会话, 拿到 conversationId 供 WS 认证使用
func prepareSession(reader *bufio.Reader) error {
	httpBase := strings.Replace(baseURL, "ws://", "http://", 1)
	client := &http.Client{Timeout: 10 * time.Second}

	authID, verifyCode, unitId := "hsdsfz2025", "123456", "683beddbdcc71f894d67e3b3"
	// customUser 保持 false, 不走交互式输入

	// 1. 登录获取 token
	body, err := postJSON(client, httpBase+"/user/sign_in", map[string]string{
		"authType":   "code-password",
		"authId":     authID,
		"verifyCode": verifyCode,
		"unitId":     unitId,
	}, "")
	if err != nil {
		return fmt.Errorf("登录请求失败: %w", err)
	}
	var signResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Token  string `json:"token"`
			UserId string `json:"userId"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &signResp); err != nil {
		return fmt.Errorf("解析登录响应失败: %w", err)
	}
	if signResp.Code != 0 {
		return fmt.Errorf("登录失败: code=%d msg=%s", signResp.Code, signResp.Msg)
	}
	authToken = signResp.Data.Token
	fmt.Println("登录成功, userId:", signResp.Data.UserId)

	// 2. 查询单位角色列表 (不鉴权接口), 取第一个启用角色作为 characterId
	body, err = getJSON(client, httpBase+"/config/get_character?unitId="+unitId, "")
	if err != nil {
		return fmt.Errorf("查询配置请求失败: %w", err)
	}
	var cfgResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Characters []struct {
				Id     string `json:"id"`
				Status int32  `json:"status"`
			} `json:"characters"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &cfgResp); err != nil {
		return fmt.Errorf("解析配置响应失败: %w", err)
	}
	if cfgResp.Code != 0 {
		return fmt.Errorf("查询配置失败: code=%d msg=%s", cfgResp.Code, cfgResp.Msg)
	}
	var characterId string
	for _, ch := range cfgResp.Data.Characters {
		if ch.Status == 1 { // ConfigStatusActive
			characterId = ch.Id
			break
		}
	}
	if characterId == "" {
		return fmt.Errorf("单位无可启用角色")
	}
	fmt.Println("使用角色:", characterId)

	// 3. 创建会话
	body, err = postJSON(client, httpBase+"/conversation/create", map[string]string{
		"characterId": characterId,
	}, authToken)
	if err != nil {
		return fmt.Errorf("创建会话请求失败: %w", err)
	}
	var convResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ConversationId string `json:"conversationId"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &convResp); err != nil {
		return fmt.Errorf("解析会话响应失败: %w", err)
	}
	if convResp.Code != 0 {
		return fmt.Errorf("创建会话失败: code=%d msg=%s", convResp.Code, convResp.Msg)
	}
	conversationId = convResp.Data.ConversationId
	fmt.Println("创建会话成功, conversationId:", conversationId)
	return nil
}

func getJSON(client *http.Client, url string, token string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func postJSON(client *http.Client, url string, payload any, token string) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func SendAuthMessage(conn *websocket.Conn, meta *core.Meta, reader *bufio.Reader) {
	var auth core.Auth
	if !customUser {
		auth = core.Auth{
			AuthType:   "code-password",
			AuthID:     "hsdsfz2025",                                               // 华四小明
			VerifyCode: "123456",                                                   //promptInput(reader, "请输入VerifyCode: "),
			Info:       map[string]any{cst.JsonUnitID: "683beddbdcc71f894d67e3b3"}, //make(map[string]any),
		}
	} else {
		auth = core.Auth{
			AuthID:     promptInput(reader, "请输入用户ID"),
			AuthType:   string(authType2Int32[promptInput(reader, "请输入认证类型")]),
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
	// 使用启动时自动创建的会话
	auth.Info[cst.JsonConversationID] = conversationId
	// 发送消息
	if err := sendMessage(conn, meta, core.MAuth, &auth); err != nil {
		log.Println("发送认证消息失败:", err)
	} else {
		fmt.Println("认证消息发送成功")
	}
}
