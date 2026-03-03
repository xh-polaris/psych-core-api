package wordcld

import (
	"context"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"strings"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"github.com/yanyiwu/gojieba"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type WordCloudExtractor struct {
	rptMapper report.IMongoMapper
	jieba     *gojieba.Jieba
}

func NewWordCloudExtractor(rptMapper report.IMongoMapper) *WordCloudExtractor {
	return &WordCloudExtractor{
		rptMapper: rptMapper,
		jieba:     gojieba.NewJieba(),
	}
}

func (wce *WordCloudExtractor) Free() {
	wce.jieba.Free()
}

func (wce *WordCloudExtractor) FromHisMsg(msgs []*message.Message) (*core_api.Keywords, error) {
	var builder strings.Builder
	for _, msg := range msgs {
		if msg.Role == message.RoleStoI[cst.User] {
			builder.WriteString(msg.Content)
			builder.WriteString(" ")
		}
	}

	text := builder.String()
	if text == "" {
		return &core_api.Keywords{KeywordMap: make(map[string]int32), KeyTotal: 0}, nil
	}

	words := wce.jieba.Cut(text, true)
	wordCounts := make(map[string]int32)
	for _, word := range words {
		// 简单过滤一些停用词和短词
		if len(strings.TrimSpace(word)) > 1 && !isStopWord(word) {
			wordCounts[word]++
		}
	}

	return &core_api.Keywords{
		KeywordMap: wordCounts,
		KeyTotal:   int32(len(wordCounts)),
	}, nil
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

// isStopWord 简单的停用词判断
func isStopWord(word string) bool {
	// 在实际应用中，这里应该从一个更完整的停用词词典中加载
	stopWords := map[string]struct{}{
		"的": {}, "了": {}, "在": {}, "是": {}, "我": {}, "你": {}, "他": {}, "她": {}, "它": {},
		"我们": {}, "你们": {}, "他们": {}, "一个": {}, "一些": {}, "什么": {}, "怎么": {}, "这个": {}, "那个": {},
	}
	_, found := stopWords[word]
	return found
}
