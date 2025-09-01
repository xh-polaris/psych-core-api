package component

import (
	"context"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/app/volc/asr"
)

var audioPath = "../output.pcm"

func TestVolcASRApp(t *testing.T) {
	asrApp := GetASRApp()
	ctx := context.Background()

	// 打开音频文件
	file, err := os.Open(audioPath)
	if err != nil {
		t.Fatalf("无法打开音频文件: %v", err)
	}
	defer func() { _ = file.Close() }()

	// 使用WaitGroup协调goroutine
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	// 发送协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendAudio(ctx, t, asrApp, file)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		receiveResults(ctx, t, asrApp)
	}()

	// 8. 等待任务完成
	wg.Wait()
	cancel()
}

// sendAudio 发送音频数据
func sendAudio(ctx context.Context, t *testing.T, asrApp app.ASRApp, file *os.File) {
	var err error
	//var res string

	buf := make([]byte, 3200) // 每次发送3200字节（约200ms 16kHz音频）
	first, last := []byte{app.FirstASR}, []byte{app.LastASR}
	if err = asrApp.Send(ctx, first); err != nil {
		t.Errorf("first: %s", err.Error())
	}

	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := file.Read(buf)
			if err == io.EOF {
				t.Log("音频发送完成")
				goto e
			}
			if err != nil {
				t.Errorf("读取音频失败: %v", err)
				goto e
			}

			if err := asrApp.Send(ctx, buf[:n]); err != nil {
				t.Errorf("发送失败: %v", err)
				return
			}
			t.Logf("发送%d", i)
			i++
			time.Sleep(10 * time.Millisecond) // 模拟实时流
		}
	}
e:
	if err := asrApp.Send(ctx, last); err != nil {
		t.Errorf("last:%s", err.Error())
	}
}

// receiveResults 接收识别结果
func receiveResults(ctx context.Context, t *testing.T, app app.ASRApp) {
	for {
		select {
		default:
			res, err := app.Receive(ctx)
			if err != nil {
				if err == io.EOF {
					t.Log("连接正常关闭")
					return
				}
				t.Errorf("接收错误: %v", err)
				return
			}

			if len(res) > 0 {
				log.Printf("识别结果: %s", res)
			}
		}
	}
}

func GetASRApp() app.ASRApp {
	return asr.NewVcASRApp("vc-asr-test", &app.ASRSetting{
		Id:         GetTestConfig()["VCASRAppId"].(string),
		Provider:   GetTestConfig()["VCASRAppProvider"].(string),
		Url:        GetTestConfig()["VCASRAppUrl"].(string),
		AppID:      GetTestConfig()["VCASRAppAppID"].(string),
		AccessKey:  GetTestConfig()["VCASRAppAccessKey"].(string),
		ResourceId: GetTestConfig()["VCASRAppResourceId"].(string),
		Format:     GetTestConfig()["VCASRAppFormat"].(string),
		Codec:      GetTestConfig()["VCASRAppCodec"].(string),
		Rate:       GetTestConfig()["VCASRAppRate"].(int),
		Bits:       GetTestConfig()["VCASRAppBits"].(int),
		Channels:   GetTestConfig()["VCASRAppChannels"].(int),
		ModelName:  GetTestConfig()["VCASRAppModelName"].(string),
		EnablePunc: GetTestConfig()["VCASRAppEnablePunc"].(bool),
		EnableDdc:  GetTestConfig()["VCASRAppEnableDdc"].(bool),
		ResultType: GetTestConfig()["VCASRAppResultType"].(string),
	})
}
