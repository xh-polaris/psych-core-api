package component

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/app/volc/tts"
)

func TestVolcTTSApp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ttsApp := GetVolcTTSApp()

	// 1️⃣ 必须先 Dial
	if err := ttsApp.Dial(ctx); err != nil {
		t.Fatalf("[tts app] dial err: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// 2️⃣ 启动接收协程（等价于 execTTSRecv）
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				frame, last, err := ttsApp.Receive(ctx)
				if err != nil {
					t.Errorf("[tts app] receive err: %v", err)
					return
				}

				fmt.Printf("[tts app] receive frame len: %d\n", len(frame))

				if last {
					t.Log("[tts app] receive last frame")
					return
				}
			}
		}
	}()

	// 3️⃣ 发送首包
	if err := ttsApp.Send(ctx, app.FirstTTS); err != nil {
		t.Fatalf("[tts app] first send err: %v", err)
	}

	// 4️⃣ 发送文本内容
	texts := []string{
		"我是张",
		"薇老师的",
		"数字分身",
		"，你可以",
		"叫我小薇老师",
	}

	for _, str := range texts {
		if err := ttsApp.Send(ctx, str); err != nil {
			t.Fatalf("[tts app] send err: %v", err)
		}
	}

	// 5️⃣ 发送尾包
	if err := ttsApp.Send(ctx, app.LastTTS); err != nil {
		t.Fatalf("[tts app] last send err: %v", err)
	}

	// 等待接收完成
	wg.Wait()
}

func GetVolcTTSApp() app.TTSApp {
	return tts.NewVcMTTSApp("volc-test"+strconv.Itoa(rand.New(rand.NewSource(time.Now().Unix())).Int()), &app.TTSSetting{
		Provider:   GetTestConfig()["VCTTSAppProvider"].(string),
		Url:        GetTestConfig()["VCTTSAppUrl"].(string),
		AppID:      GetTestConfig()["VCTTSAppAppID"].(string),
		AccessKey:  GetTestConfig()["VCTTSAppAccessKey"].(string),
		Namespace:  GetTestConfig()["VCTTSAppNamespace"].(string),
		Speaker:    GetTestConfig()["VCTTSAppSpeaker"].(string),
		ResourceId: GetTestConfig()["VCTTSAppResourceId"].(string),
		AudioParams: &app.AudioParams{
			Format:       GetTestConfig()["VCTTSAppAudioFormat"].(string),
			Codec:        GetTestConfig()["VCTTSAppAudioCodec"].(string),
			Rate:         int32(GetTestConfig()["VCTTSAppAudioRate"].(int)),
			Bits:         int32(GetTestConfig()["VCTTSAppAudioBits"].(int)),
			Channels:     GetTestConfig()["VCTTSAppAudioChannels"].(int),
			SpeechRate:   int32(GetTestConfig()["VCTTSAppAudioSpeechRate"].(int)),
			LoudnessRate: int32(GetTestConfig()["VCTTSAppAudioLoudnessRate"].(int)),
			Lang:         GetTestConfig()["VCTTSAppAudioLang"].(string),
			ResultType:   GetTestConfig()["VCTTSAppAudioResultType"].(string),
		},
	})
}
