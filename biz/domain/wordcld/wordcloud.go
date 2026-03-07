package wordcld

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"github.com/yanyiwu/gojieba"
	"go.mongodb.org/mongo-driver/v2/bson"
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

// 内置默认停用词表
var defaultStopWords = []string{
	"的", "了", "在", "是", "我", "你", "他", "她", "它",
	"我们", "你们", "他们", "一个", "一些", "什么", "怎么", "这个", "那个",
	"有", "没有", "会", "不会", "可以", "不可以", "能", "不能",
	"很", "非常", "特别", "真的", "确实", "应该", "可能", "或者",
	"但是", "不过", "然后", "所以", "因为", "如果", "虽然", "虽说",
	"就是", "只是", "还是", "还有", "而且", "并且", "或", "和",
	"啊", "呀", "哦", "嗯", "呢", "吧", "吗", "呗", "哈", "嘿",
}

// loadStopWords 加载停用词列表
func loadStopWords() {
	stopWords = make(map[string]struct{})

	// 尝试从配置文件加载
	stopWordsPath := os.Getenv("STOPWORDS_PATH")
	if stopWordsPath == "" {
		stopWordsPath = "etc/stopwords.txt"
	}

	// 尝试相对于工作目录和可执行文件目录
	paths := []string{
		stopWordsPath,
		filepath.Join("etc", "stopwords.txt"),
	}

	loaded := false
	for _, path := range paths {
		if err := loadStopWordsFromFile(path); err == nil {
			loaded = true
			break
		}
	}

	// 如果没有成功从文件加载，使用默认停用词表
	if !loaded {
		for _, word := range defaultStopWords {
			stopWords[strings.TrimSpace(word)] = struct{}{}
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
		if word != "" && !strings.HasPrefix(word, "#") { // 支持注释行
			stopWords[word] = struct{}{}
		}
	}

	// 添加默认停用词以确保基本覆盖
	for _, word := range defaultStopWords {
		stopWords[strings.TrimSpace(word)] = struct{}{}
	}

	return scanner.Err()
}

// ensureStopWordsLoaded 确保停用词已加载
func ensureStopWordsLoaded() {
	stopWordsOnce.Do(loadStopWords)
}

func NewWordCloudExtractor(rptMapper report.IMongoMapper) *WordCloudExtractor {
	Extractor = WordCloudExtractor{
		rptMapper: rptMapper,
		jieba:     gojieba.NewJieba(),
	}
	return &Extractor
}

func (wce *WordCloudExtractor) Free() {
	wce.jieba.Free()
}

func (wce *WordCloudExtractor) FromHisMsg(msgs []*message.Message) (*core_api.Keywords, error) {
	var builder strings.Builder
	for _, msg := range msgs {
		if msg.Role == message.RoleStoI[cst.User] {
			// 预处理消息内容：去除多余空白和标点
			content := preprocessText(msg.Content)
			if content != "" {
				builder.WriteString(content)
				builder.WriteString(" ")
			}
		}
	}

	text := strings.TrimSpace(builder.String())
	if text == "" {
		return &core_api.Keywords{KeywordMap: make(map[string]int32), KeyTotal: 0}, nil
	}

	// 使用结巴分词
	words := wce.jieba.Cut(text, true)
	wordCounts := make(map[string]int32)

	for _, word := range words {
		// 标准化词语
		normalizedWord := normalizeWord(word)

		// 过滤无效词语
		if isValidWord(normalizedWord) {
			wordCounts[normalizedWord]++
		}
	}

	return &core_api.Keywords{
		KeywordMap: wordCounts,
		KeyTotal:   int32(len(wordCounts)),
	}, nil
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

func (wce *WordCloudExtractor) FromUnitKWs(ctx context.Context, unitId bson.ObjectID) (*core_api.Keywords, error) {
	kws, err := wce.rptMapper.GetUnitKW(ctx, unitId)
	if err != nil {
		return nil, err
	}
	if kws == nil {
		kws = make(map[string]int32)
	}
	return &core_api.Keywords{
		KeywordMap: kws,
		KeyTotal:   int32(len(kws)),
	}, nil
}

func (wce *WordCloudExtractor) FromAllUnitsKWs(ctx context.Context) (*core_api.Keywords, error) {
	kws, err := wce.rptMapper.GetAllUnitsKW(ctx)
	if err != nil {
		return nil, err
	}
	if kws == nil {
		kws = make(map[string]int32)
	}
	return &core_api.Keywords{
		KeywordMap: kws,
		KeyTotal:   int32(len(kws)),
	}, nil
}

// isStopWord 判断是否为停用词
func isStopWord(word string) bool {
	ensureStopWordsLoaded()
	_, found := stopWords[word]
	return found
}
