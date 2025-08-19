package component

import (
	"context"
	"fmt"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/app/volc"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

type TestTTSPipe struct {
	ctx        context.Context
	unexpected func(error)

	in   *core.Channel[*core.Cmd] // 命令输入
	test chan *core.Cmd
	out  *core.Channel[*core.Resp] // 输出
}

func NewTestTTSPipe(ctx context.Context, unexpected func(error), close chan struct{}, out *core.Channel[*core.Resp]) *TestTTSPipe {
	return &TestTTSPipe{
		ctx:        ctx,
		unexpected: unexpected,
		out:        out,
		test:       make(chan *core.Cmd),
		in:         core.NewChannel[*core.Cmd](3, close),
	}
}

// In 上传text, 由in关闭
func (p *TestTTSPipe) In() {
	for cmd := range p.in.C {
		p.test <- cmd
		logx.Info("[tts pipe] send cmd:%v", cmd)
	}

}

// Out 获取audio, 由out关闭
func (p *TestTTSPipe) Out() {
	for cmd := range p.test {
		resp := &core.Resp{
			ID:      0, // Optimize tts输出应该也和cmd的ID对应上
			Type:    core.RModelAudio,
			Content: cmd.Content,
		}
		p.out.Send(resp)
	}
}

func (p *TestTTSPipe) Run() {
	go p.In()
	go p.Out()
}

func (p *TestTTSPipe) Close() {
	p.in.Close()
}

func TestVolcTTSApp(t *testing.T) {
	strs := []string{
		app.FirstTTS,
		"我是张",
		"薇老师的",
		"数字分身",
		"，你可以",
		"叫我小薇老师",
		app.LastTTS,
		app.FirstTTS,
		"我是张",
		"薇老师的",
		"数字分身",
		"，你可以",
		"叫我小薇老师",
		app.LastTTS,
	}
	ctx := context.Background()
	closeCh := make(chan struct{})
	tts := GetVolcTTSApp()
	go func() { // 接受
		for {
			select {
			case <-closeCh:
				return
			default:
				frame, err := tts.Receive(ctx)
				if err != nil {
					t.Errorf("[tts app] receive err:%s", err)
					return
				}
				fmt.Printf("[tts app] receive frame:%+v\n", frame)
			}
		}
	}()
	for _, str := range strs {
		if str == app.FirstTTS {
			time.Sleep(3 * time.Second)
		}
		if err := tts.Send(ctx, str); err != nil {
			t.Errorf("[tts app] send err:%s", err)
			time.Sleep(100 * time.Millisecond)
		}
	}
	time.Sleep(30 * time.Second)

}

func GetVolcTTSApp() app.TTSApp {
	return volc.NewVcMTTSApp("volc-test"+strconv.Itoa(rand.New(rand.NewSource(time.Now().Unix())).Int()), &app.TTSSetting{
		Id:         GetTestConfig()["VCTTSAppId"].(string),
		Provider:   GetTestConfig()["VCTTSAppProvider"].(string),
		Url:        GetTestConfig()["VCTTSAppUrl"].(string),
		AppID:      GetTestConfig()["VCTTSAppAppID"].(string),
		AccessKey:  GetTestConfig()["VCTTSAppAccessKey"].(string),
		Namespace:  GetTestConfig()["VCTTSAppNamespace"].(string),
		Speaker:    GetTestConfig()["VCTTSAppSpeaker"].(string),
		ResourceId: GetTestConfig()["VCTTSAppResourceId"].(string),
		AudioParams: struct {
			Format       string `json:"format"`
			Codec        string `json:"codec"`
			Rate         int32  `json:"rate"`
			Bits         int32  `json:"bits"`
			Channels     int    `json:"channels"`
			SpeechRate   int32  `json:"speech_rate"`
			LoudnessRate int32  `json:"loudness_rate"`
			Lang         string `json:"lang"`
			ResultType   string `json:"result_type"`
		}{
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
