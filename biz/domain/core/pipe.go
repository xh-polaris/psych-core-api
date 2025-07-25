package core

// ASRBiPipe 处理ASR的Pipe
type ASRBiPipe struct {
	// in 输入, 可能是文字也可能是音频
	in chan []byte
	// out 输出, 识别的文字
	out chan string
}

func (p *ASRBiPipe) Run() {

}
