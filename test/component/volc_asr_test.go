package component

import (
	"context"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/app/volc/asr"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
)

var audioPath = "../slice.pcm"

func TestVolcASRApp(t *testing.T) {
	asrApp := GetASRApp(t)
	t.Logf("asr app: %v", asrApp)

	file, err := os.Open(audioPath)
	if err != nil {
		t.Fatalf("无法打开音频文件: %v", err)
	}
	defer file.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 建立连接
	if err = asrApp.Dial(ctx); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	// 启动接收协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		receiveResults(ctx, t, asrApp)
	}()

	// 启动发送协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendAudio(ctx, t, asrApp, file)
	}()

	wg.Wait()
}

func sendAudio(ctx context.Context, t *testing.T, asrApp app.ASRApp, file *os.File) {

	buf := make([]byte, 3200)
	last := []byte{app.LastASR}

	i := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := file.Read(buf)
			if err == io.EOF {
				t.Log("音频发送完成")
				goto END
			}
			if err != nil {
				t.Errorf("读取音频失败: %v", err)
				goto END
			}
			if err := asrApp.Send(ctx, buf[:n]); err != nil {
				t.Errorf("发送失败: %v", err)
				return
			}
			t.Logf("发送 chunk %d", i)
			i++
			// 模拟实时音频
			time.Sleep(200 * time.Millisecond)
		}
	}

END:

	if err := asrApp.Send(ctx, last); err != nil {
		t.Errorf("发送 last 失败: %v", err)
	}
	t.Log("发送Last")
}

func receiveResults(ctx context.Context, t *testing.T, app app.ASRApp) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			res, end, definite, err := app.Receive(ctx)
			if err != nil {
				if !wsx.IsNormal(err) && !websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
					t.Log("连接正常关闭")
					return
				}
				t.Errorf("接收错误: %v", err)
				return
			}
			if len(res) > 0 {
				log.Printf("是否分句 %v, 识别结果: %s", definite, res)
			}
			if definite {
				t.Log("分句")
				return
			}
			if end {
				t.Log("识别结束")
				return
			}
		}
	}
}

func GetASRApp(t *testing.T) app.ASRApp {
	setting := &app.ASRSetting{
		Provider:           GetTestConfig()["VCASRAppProvider"].(string),
		Url:                GetTestConfig()["VCASRAppUrl"].(string),
		AppID:              GetTestConfig()["VCASRAppAppID"].(string),
		AccessKey:          GetTestConfig()["VCASRAppAccessKey"].(string),
		ResourceId:         GetTestConfig()["VCASRAppResourceId"].(string),
		Format:             GetTestConfig()["VCASRAppFormat"].(string),
		Codec:              GetTestConfig()["VCASRAppCodec"].(string),
		Rate:               GetTestConfig()["VCASRAppRate"].(int),
		Bits:               GetTestConfig()["VCASRAppBits"].(int),
		Channels:           GetTestConfig()["VCASRAppChannels"].(int),
		ModelName:          GetTestConfig()["VCASRAppModelName"].(string),
		EnablePunc:         GetTestConfig()["VCASRAppEnablePunc"].(bool),
		EnableDdc:          GetTestConfig()["VCASRAppEnableDdc"].(bool),
		ResultType:         GetTestConfig()["VCASRAppResultType"].(string),
		ShowUtterances:     GetTestConfig()["VCASRAppShowUtterances"].(bool),
		VADSegmentDuration: GetTestConfig()["VCASRAppVADSegmentDuration"].(int),
		EndWindowSize:      GetTestConfig()["VCASRAppEndWindowSize"].(int),
	}
	t.Logf("asr setting: %v", setting)
	return asr.NewVcASRApp("", setting)
}
