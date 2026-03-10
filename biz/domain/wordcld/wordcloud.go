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

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/pkg/filter"
	"github.com/yanyiwu/gojieba"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var Extractor WordCloudExtractor

type WordCloudExtractor struct {
	rptMapper report.IMongoMapper
	jieba     *gojieba.Jieba
}

var (
	// 全局过滤引擎
	dfaFilter     *filter.DFAFilter
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
	"好的", "好的吧", "明白", "知道", "好吧", "没问题", "确实是",
}

// 词性黑名单 (针对虚词、代词等)
var posBlacklist = map[string]struct{}{
	"x":  {}, // 非语素词
	"zg": {}, // 状态词
	"uj": {}, // 助词
	"ul": {}, // 助词
	"uv": {}, // 助词
	"uz": {}, // 助词
	"ug": {}, // 助词
	"r":  {}, // 代词
	"c":  {}, // 连词
	"p":  {}, // 介词
	"u":  {}, // 助词
	"y":  {}, // 语气词
	"e":  {}, // 叹词
	"o":  {}, // 拟声词
	"m":  {}, // 数词
	"q":  {}, // 量词
}

// loadStopWords 加载并初始化 DFA 过滤器
func loadStopWords() {
	dfaFilter = filter.NewDFAFilter()

	// 1. 加载默认停用词
	for _, word := range defaultStopWords {
		dfaFilter.AddWord(strings.TrimSpace(word))
	}

	// 2. 尝试从配置文件加载
	stopWordsPath := os.Getenv("STOPWORDS_PATH")
	if stopWordsPath == "" {
		stopWordsPath = "etc/stopwords.txt"
	}

	paths := []string{
		stopWordsPath,
		filepath.Join("etc", "stopwords.txt"),
	}

	for _, path := range paths {
		if err := loadStopWordsFromFile(path); err == nil {
			break
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
			dfaFilter.AddWord(word)
		}
	}

	return scanner.Err()
}

// ensureStopWordsLoaded 确保过滤器已初始化
func ensureStopWordsLoaded() {
	stopWordsOnce.Do(loadStopWords)
}

func NewWordCloudExtractor(rptMapper report.IMongoMapper) *WordCloudExtractor {
	ensureStopWordsLoaded()
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
	if len(msgs) == 0 {
		return &core_api.Keywords{KeywordMap: make(map[string]int32), KeyTotal: 0}, nil
	}

	wordCounts := make(map[string]int32)
	for _, msg := range msgs {
		if msg.Role != message.RoleStoI[cst.User] {
			continue
		}

		// 预处理
		content := preprocessText(msg.Content)
		if content == "" {
			continue
		}

		// 使用词性标注
		tags := wce.jieba.Tag(content)
		for _, tag := range tags {
			parts := strings.Split(tag, "/")
			if len(parts) != 2 {
				continue
			}
			word, pos := parts[0], parts[1]

			// 标准化
			word = normalizeWord(word)

			// 组合校验：1.词性校验 2.合法性校验 3.DFA停用词/敏感词校验
			if isPOSAllowed(pos) && isValidWord(word) {
				wordCounts[word]++
			}
		}
	}

	if len(wordCounts) == 0 {
		return &core_api.Keywords{KeywordMap: make(map[string]int32), KeyTotal: 0}, nil
	}

	return &core_api.Keywords{
		KeywordMap: wordCounts,
		KeyTotal:   int32(len(wordCounts)),
	}, nil
}

// isPOSAllowed 词性校验
func isPOSAllowed(pos string) bool {
	if _, black := posBlacklist[pos]; black {
		return false
	}
	return true
}

// preprocessText 预处理文本内容
func preprocessText(text string) string {
	if text == "" {
		return ""
	}
	// 移除标点
	text = punctuationRegex.ReplaceAllString(text, " ")
	// 标准化空白
	text = whitespaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// normalizeWord 标准化词语
func normalizeWord(word string) string {
	word = strings.TrimSpace(word)
	word = strings.ToLower(word)
	return word
}

// isValidWord 判断词语是否有效 (常规校验)
func isValidWord(word string) bool {
	if word == "" {
		return false
	}

	// 长度检查：过滤单字 (除非是特定的实词，但在词云中单字通常意义不大)
	if utf8.RuneCountInString(word) < 2 {
		return false
	}

	// 纯数字检查
	if strings.TrimFunc(word, func(r rune) bool {
		return (r >= '0' && r <= '9') || r == '.' || r == ','
	}) == "" {
		return false
	}

	// DFA 停用词校验
	if dfaFilter.IsMatched(word) {
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
