package component

import (
	"context"
	"errors"
	"fmt"
	"github.com/xh-polaris/psych-core-api/biz/domain/workflow"
	"github.com/xh-polaris/psych-pkg/app"
	bailian "github.com/xh-polaris/psych-pkg/app/bailian"
	"github.com/xh-polaris/psych-pkg/core"
	"testing"
)

// bailian app 测试

func TestBaiLianChatApp_StreamCall(t *testing.T) {
	chat := GetBLChatApp()
	scanner, err := chat.StreamCall(context.Background(), "你是谁", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scanner.Close() }()
	for {
		data, err := scanner.Next()
		if err != nil {
			if errors.Is(err, app.End) {
				break
			}
			t.Fatal(err)
		}
		fmt.Println(data)
	}
}

func TestBaiLianChatPipe(t *testing.T) {
	// 必要的channel
	ctx := context.Background()
	closeChan := make(chan struct{})
	hisChan := core.NewChannel[*core.HisEntry](3, closeChan)
	outChan := core.NewChannel[*core.Resp](3, closeChan)
	unexcepted := func(err error) {
		t.Fatalf("Unexcepted error: %s", err.Error())
	}
	ttsPipe := NewTestTTSPipe(ctx, unexcepted, closeChan, outChan)
	ttsPipe.Run()
	ttsInF, err := GetIn(ttsPipe)
	if err != nil {
		t.Fatal(err)
	}
	ttsIn := ttsInF.(*core.Channel[*core.Cmd])
	// 基于bailian的chat pipe
	chatPipe := workflow.NewChatPipe(context.Background(), unexcepted, closeChan, GetBLChatApp(), "bailian-test", hisChan, ttsIn, outChan)
	chatPipe.Run()
	hisPipe := NewTestHistoryPipe(closeChan, "bailian-test")
	hisPipe.Run()

	filed, err := GetIn(chatPipe)
	if err != nil {
		t.Fatal(err)
	}
	in := filed.(*core.Channel[*core.Cmd])

	// 输入
	go func() {
		in.Send(&core.Cmd{
			ID:      0,
			Role:    "test",
			Command: core.CUserText,
			Content: "你是谁",
		})
	}()
	Out(outChan) // 输出
	close(closeChan)
}

func GetBLChatApp() app.ChatApp {
	return bailian.NewBLChatApp("test", &app.ChatSetting{
		Id:        GetTestConfig()["BLChatAppId"].(string),
		Provider:  GetTestConfig()["BLChatAppProvider"].(string),
		Url:       GetTestConfig()["BLChatAppUrl"].(string),
		AppId:     GetTestConfig()["BLChatAppAppId"].(string),
		AccessKey: GetTestConfig()["BLChatAppAccessKey"].(string),
	})
}
