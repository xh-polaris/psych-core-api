package wordcld

import (
	"context"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type WordCloudExtractor struct {
	rptMapper report.IMongoMapper
}

func (wce *WordCloudExtractor) FromHisMsg(msgs []*message.Message) ([]*core_api.Keywords, error) {
	return nil, nil
}

func (wce *WordCloudExtractor) FromUnitKWs(ctx context.Context, unitId bson.ObjectID) ([]*core_api.Keywords, error) {
	return nil, nil
}

func (wce *WordCloudExtractor) FromAllUnitsKWs(ctx context.Context) ([]*core_api.Keywords, error) {
	return nil, nil
}
