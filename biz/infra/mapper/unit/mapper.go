package unit

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var _ IMongoMapper = (*mongoMapper)(nil)

const (
	prefixUnitCacheKey = "cache:unit"
	collectionName     = "unit"
)

type IMongoMapper interface {
	FindOneByPhone(ctx context.Context, phone string) (*Unit, error)
	FindOne(ctx context.Context, id primitive.ObjectID) (*Unit, error)
	Insert(ctx context.Context, unit *Unit) error
	UpdateFields(ctx context.Context, id primitive.ObjectID, update bson.M) error
	ExistsByPhone(ctx context.Context, phone string) (bool, error)
}

type mongoMapper struct {
	mapper.IMongoMapper[Unit]
	conn *monc.Model
}

func NewUnitMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		IMongoMapper: mapper.NewMongoMapper[Unit](conn),
		conn:         conn,
	}
}

// FindOneByPhone 根据手机号查询单位
func (m *mongoMapper) FindOneByPhone(ctx context.Context, phone string) (*Unit, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Phone: phone})
}

// ExistsByPhone 根据手机号查询单位是否存在
func (m *mongoMapper) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Phone: phone})
}
