// Copyright © 2025 univero. All rights reserved.
// Licensed under the GNU Affero General Public License v3 (AGPL-3.0).
// license that can be found in the LICENSE file.

package asr

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"

	"golang.org/x/net/context"
)

var _ app.ASRApp = (*VcASRApp)(nil)

func init() {
	app.ASRRegister("volc", NewVcASRApp)
}

// VcASRApp 是火山引擎的大模型语音识别
// 前后端一个长连接, 每轮对话, 收到first后建立新的asr链接, 每次发送first前需要保证前一次的last已发送
// 双向流式会增量返回, 流式则是最后一个包或15s后返回, 单包时长100~200ms最优
type VcASRApp struct {
	wsx *wsx.WSClient

	// 鉴权与配置
	setting *app.ASRSetting

	// seq 发送的消息序列号
	seq int
	// session
	uSession string
	// header 是请求头, 携带鉴权信息
	header http.Header
}

// NewVcASRApp 构造一个新的ASR App
func NewVcASRApp(uSession string, setting *app.ASRSetting) app.ASRApp {
	asr := &VcASRApp{
		setting:  setting,
		seq:      1,
		uSession: uSession,
	}
	asr.buildHTTPHeader()
	return asr
}

// Dial 建立ws链接
func (asr *VcASRApp) Dial(ctx context.Context) (err error) {
	asr.wsx, err = wsx.NewWSClientWithDial(ctx, asr.setting.Url, asr.header)
	if err != nil {
		return err
	}
	asr.seq = 1
	return asr.start()
}

// start 完成应用层协议握手
func (asr *VcASRApp) start() (err error) {
	var payload []byte
	setting := asr.setting
	// 协商配置参数
	req := NewFullClientRequest(asr.uSession, setting.Format, setting.Codec, setting.Rate, setting.Bits,
		setting.Channels, setting.ModelName, true, setting.EnablePunc, setting.EnableDdc, setting.ResultType, setting.ShowUtterances, false,
		setting.VADSegmentDuration, setting.EndWindowSize)
	if payload, err = json.Marshal(req); err != nil {
		return err
	}
	if payload, err = util.GzipCompress(payload); err != nil {
		return err
	}
	// 组装full client request, full client request = header + sequence + payload
	seq := util.I2BigEndBytes(asr.seq)
	size := util.I2BigEndBytes(len(payload))
	fullClientRequest := util.BuildBytes(PosDefaultHeader, seq, size, payload)
	if err = asr.wsx.WriteBytes(fullClientRequest); err != nil {
		return err
	}
	asr.seq++
	return
}

// Send 发送音频流
func (asr *VcASRApp) Send(ctx context.Context, data []byte) (err error) {
	var payload, header []byte
	if app.IsFirstASR(data) {
		return
	}

	header = AudioPosDefaultHeader
	if app.IsLastASR(data) { // 判断是否最后一个包, 若是则负载为空, 序号为负
		header = AudioNegDefaultHeader
		asr.seq = -asr.seq
	} else {
		payload, err = util.GzipCompress(data)
		if err != nil {
			return err
		}
	}

	// 发送音频流
	seq := util.I2BigEndBytes(asr.seq)
	size := util.I2BigEndBytes(len(payload))
	audioOnlyRequest := util.BuildBytes(header, seq, size, payload)
	if err = asr.wsx.WriteBytes(audioOnlyRequest); err != nil {
		return err
	}
	asr.seq++
	return nil
}

// Receive 接受响应
func (asr *VcASRApp) Receive(_ context.Context) (text string, last bool, definite bool, err error) {
	var msg []byte
	var mt int
	if mt, msg, err = asr.wsx.Read(); err == nil {
		switch mt {
		case websocket.BinaryMessage:
			resp := ParseResponse(msg)
			if len(resp.PayloadMsg.Result.Utterances) > 0 {
				for _, u := range resp.PayloadMsg.Result.Utterances {
					if u.Definite {
						definite = true
					}
				}
			}
			return resp.PayloadMsg.Result.Text, resp.IsLastPackage, definite, nil
		case websocket.TextMessage:
			return asr.receiveText(msg)
		default:
			return "", false, false, err
		}
	}
	return "", false, false, err
}

// receiveText 接受到文本消息, 暂无实际用途
func (asr *VcASRApp) receiveText(res []byte) (text string, last bool, definite bool, err error) {
	logs.Info("[volc asr] receiveText: ", string(res))
	return string(res), false, false, nil
}

// Close 释放资源
func (asr *VcASRApp) Close() (err error) {
	if asr.wsx != nil {
		return asr.wsx.Close()
	}
	return
}

// buildHTTPHeader 构造鉴权请求头
func (asr *VcASRApp) buildHTTPHeader() {
	asr.header = http.Header{
		"X-Api-Resource-Id": []string{asr.setting.ResourceId},
		"X-Api-Access-Key":  []string{asr.setting.AccessKey},
		"X-Api-App-Key":     []string{asr.setting.AppID},
		"X-Api-Request-Id":  []string{asr.uSession},
	}
}
