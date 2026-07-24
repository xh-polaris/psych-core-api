package voice

import (
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"
	"github.com/zeromicro/go-zero/core/stores/monc"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const collectionName = "voice"

type IMongoMapper interface {
	mapper.IMongoMapper[Voice]
}

type mongoMapper struct {
	mapper.IMongoMapper[Voice]
	conn *monc.Model
}

func NewVoiceMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		IMongoMapper: mapper.NewMongoMapper[Voice](conn),
		conn:         conn,
	}
}
