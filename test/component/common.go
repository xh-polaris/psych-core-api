package component

import (
	"bufio"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

var TestConfig map[string]any
var onceConfig sync.Once
var configPath = "../test_config.yaml"

// GetTestConfig 获取测试配置
func GetTestConfig() map[string]any {
	onceConfig.Do(func() {
		var data []byte
		var err error
		TestConfig = make(map[string]any)
		// 读取文件内容
		if data, err = os.ReadFile(configPath); err != nil {
			panic("[test config] read file err:" + err.Error())
		}
		// 解析YAML到map
		if err = yaml.Unmarshal(data, &TestConfig); err != nil {
			panic("[test config] unmarshal err:" + err.Error())
		}
	})
	return TestConfig
}

func GetIn(obj any) (any, error) {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("输入必须是结构体或结构体指针")
	}
	field := val.FieldByName("in")
	if !field.IsValid() {
		return nil, fmt.Errorf("不存在") // 没有 in 字段
	}
	// 处理未导出字段
	if !field.CanInterface() {
		// 使用 unsafe 获取未导出字段的值
		return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface(), nil
	}
	return field.Interface(), nil
}

func GetUserInput() string {
	reader := bufio.NewReader(os.Stdin)
	for reader.Buffered() > 0 {
		_, _ = reader.ReadString('\n')
	}
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
