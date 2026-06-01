package wordcld

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/xh-polaris/psych-core-api/types/enum"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/yanyiwu/gojieba"
)

var Extractor WordCloudExtractor

type WordCloudExtractor struct {
	rptMapper report.IMongoMapper
	jieba     *gojieba.Jieba
}

var (
	// 全局停用词集合
	stopWords     map[string]struct{}
	stopWordsOnce sync.Once

	// 文本清理正则表达式
	punctuationRegex = regexp.MustCompile(`[^\p{Han}\p{L}\p{N}]+`)
	whitespaceRegex  = regexp.MustCompile(`\s+`)
)

// loadStopWords 加载停用词列表
func loadStopWords() {
	stopWords = make(map[string]struct{})

	// 尝试从配置文件加载
	stopWordsPath := os.Getenv("STOPWORDS_PATH")
	if stopWordsPath == "" {
		stopWordsPath = "etc/stopwords_full.txt"
	}

	// 尝试相对于工作目录和可执行文件目录
	paths := []string{
		stopWordsPath,
		filepath.Join("etc", "stopwords_full.txt"),
	}

	for _, path := range paths {
		if err := loadStopWordsFromFile(path); err == nil {
			return
		}
	}
}

// loadStopWordsFromFile 从文件加载停用词
func loadStopWordsFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" && !strings.HasPrefix(word, "#") {
			stopWords[word] = struct{}{}
		}
	}

	return scanner.Err()
}

// ensureStopWordsLoaded 确保停用词已加载
func ensureStopWordsLoaded() {
	stopWordsOnce.Do(loadStopWords)
}

// initJiebaInstance 初始化jieba实例
func initJiebaInstance() *gojieba.Jieba {
	dictPath := os.Getenv("JIEBA_DICT_PATH")

	// 在生产环境（Docker）中，必须使用自定义路径，因为默认的Go模块路径不存在
	// 如果没有设置环境变量，设置默认值为Docker中的字典路径
	if dictPath == "" {
		dictPath = "/app/dict"
	}

	// 检查自定义字典目录是否存在并包含必要的字典文件
	requiredFiles := []string{
		"jieba.dict.utf8",
		"hmm_model.utf8",
		"user.dict.utf8",
		"idf.utf8",
		"stop_words.utf8",
	}

	// 检查所有字典文件是否存在
	dictPaths := make([]string, 0, len(requiredFiles))
	for _, filename := range requiredFiles {
		fullPath := filepath.Join(dictPath, filename)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// 如果字典文件不存在，尝试使用gojieba的默认配置（仅限开发环境）
			// 生产环境中这通常会失败，所以应该确保字典文件正确部署
			return gojieba.NewJieba()
		}
		dictPaths = append(dictPaths, fullPath)
	}

	// 如果所有字典文件都存在，使用自定义路径
	return gojieba.NewJieba(dictPaths...)
}

func NewWordCloudExtractor(rptMapper report.IMongoMapper) *WordCloudExtractor {
	Extractor = WordCloudExtractor{
		rptMapper: rptMapper,
		jieba:     initJiebaInstance(),
	}
	return &Extractor
}

func (wce *WordCloudExtractor) Free() {
	wce.jieba.Free()
}

// WordCountResult 词云计数+总词数
type WordCountResult struct {
	Total int32            `json:"total"`
	Items map[string]int32 `json:"items"`
}

// FromHisMsgCount 仅返回次数版结果：包含总词数和各词出现次数。
func (wce *WordCloudExtractor) FromHisMsgCount(msgs []*message.Message) (*WordCountResult, error) {
	var builder strings.Builder
	for _, msg := range msgs {
		if msg.Role == enum.MsgRoleUser {
			content := preprocessText(msg.Content)
			if content != "" {
				builder.WriteString(content)
				builder.WriteString(" ")
			}
		}
	}

	text := strings.TrimSpace(builder.String())
	if text == "" {
		return &WordCountResult{Total: 0, Items: make(map[string]int32)}, nil
	}

	// 使用 TF-IDF 提取关键词（top 50）
	words := wce.jieba.Extract(text, 50)
	wordCounts := make(map[string]int32)
	for _, word := range words {
		normalizedWord := normalizeWord(word)
		if isValidWord(normalizedWord) {
			wordCounts[normalizedWord]++
		}
	}

	return &WordCountResult{Total: int32(len(wordCounts)), Items: wordCounts}, nil
}

// FromHisMsgPercent 返回百分比版词云结果，值保留两位小数（百分比），不包含 total。
func (wce *WordCloudExtractor) FromHisMsgPercent(msgs []*message.Message) (map[string]float64, error) {
	counts, err := wce.FromHisMsgCount(msgs)
	if err != nil {
		return nil, err
	}
	pct := make(map[string]float64, len(counts.Items))
	if counts.Total == 0 {
		return pct, nil
	}
	// 先按两位小数四舍五入计算每项百分比，并累加
	keys := make([]string, 0, len(counts.Items))
	for k := range counts.Items {
		keys = append(keys, k)
	}

	var sum float64
	for i, k := range keys {
		v := counts.Items[k]
		raw := float64(v) / float64(counts.Total) * 100.0
		if i == len(keys)-1 {
			// 最后一项用剩余补齐，避免累计误差
			last := math.Round((100.0-sum)*100) / 100
			if last < 0 {
				last = 0
			}
			pct[k] = last
		} else {
			rounded := math.Round(raw*100) / 100
			pct[k] = rounded
			sum += rounded
		}
	}

	// 最后再做一次轻量归一化调整（按比例缩放并在末项补齐），以防仍有微小误差
	var sumCheck float64
	for _, v := range pct {
		sumCheck += v
	}
	if math.Abs(sumCheck-100.0) > 1e-6 {
		factor := 100.0 / sumCheck
		var acc float64
		for i, k := range keys {
			if i == len(keys)-1 {
				last := math.Round((100.0-acc)*100) / 100
				if last < 0 {
					last = 0
				}
				pct[k] = last
			} else {
				newv := math.Round((pct[k]*factor)*100) / 100
				pct[k] = newv
				acc += newv
			}
		}
	}

	return pct, nil
}

// 兼容旧调用：FromHisMsg 同时返回百分比和次数
func (wce *WordCloudExtractor) FromHisMsg(msgs []*message.Message) (map[string]float64, *WordCountResult, error) {
	pct, err := wce.FromHisMsgPercent(msgs)
	if err != nil {
		return nil, nil, err
	}
	counts, err := wce.FromHisMsgCount(msgs)
	if err != nil {
		return nil, nil, err
	}
	return pct, counts, nil
}

// preprocessText 预处理文本内容
func preprocessText(text string) string {
	if text == "" {
		return ""
	}

	// 移除多余的标点符号，保留中文、字母和数字
	text = punctuationRegex.ReplaceAllString(text, " ")

	// 标准化空白字符
	text = whitespaceRegex.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// normalizeWord 标准化词语
func normalizeWord(word string) string {
	word = strings.TrimSpace(word)
	word = strings.ToLower(word) // 转为小写（对英文有效）
	return word
}

// isValidWord 判断词语是否有效
func isValidWord(word string) bool {
	// 空词检查
	if word == "" {
		return false
	}

	// 长度检查：过滤过短的词
	if utf8.RuneCountInString(word) < 2 {
		return false
	}

	// 纯数字检查
	if strings.TrimFunc(word, func(r rune) bool {
		return (r >= '0' && r <= '9') || r == '.' || r == ','
	}) == "" {
		return false
	}

	// 停用词检查
	if isStopWord(word) {
		return false
	}

	return true
}

// isStopWord 判断是否为停用词
func isStopWord(word string) bool {
	ensureStopWordsLoaded()
	_, found := stopWords[word]
	return found
}
