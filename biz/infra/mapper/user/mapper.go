package user

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
	prefixUserCacheKey = "cache:user"
	collectionName     = "user"
)

type IMongoMapper interface {
	FindOneByCode(ctx context.Context, phone string) (*User, error)
	FindOneByCodeAndUnitID(ctx context.Context, phone string, unitId primitive.ObjectID) (*User, error)
	FindOne(ctx context.Context, id primitive.ObjectID) (*User, error)
	Insert(ctx context.Context, user *User) error
	UpdateFields(ctx context.Context, id primitive.ObjectID, update bson.M) error
	ExistsByCode(ctx context.Context, phone string) (bool, error)
	ExistsByCodeAndUnitID(ctx context.Context, code string, unitID primitive.ObjectID) (bool, error)
	FindAllByUnitID(ctx context.Context, unitId primitive.ObjectID) ([]*User, error)
}

type mongoMapper struct {
	mapper.IMongoMapper[User]
	conn *monc.Model
}

func NewUserMongoMapper(config *conf.Config) IMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collectionName, config.CacheConf)
	return &mongoMapper{
		IMongoMapper: mapper.NewMongoMapper[User](conn),
		conn:         conn,
	}
}

// FindOneByCode 根据电话号码或学号查询用户
func (m *mongoMapper) FindOneByCode(ctx context.Context, code string) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Code: code})
}

// FindOneByCodeAndUnitID 根据电话号码和UnitID查询用户
func (m *mongoMapper) FindOneByCodeAndUnitID(ctx context.Context, code string, unitId primitive.ObjectID) (*User, error) {
	return m.FindOneByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitId})
}

// ExistsByCode 根据电话号码或学号查询用户是否存在
func (m *mongoMapper) ExistsByCode(ctx context.Context, code string) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Code: code})
}

// ExistsByCodeAndUnitID 根据电话号码和UnitID查询用户是否存在
func (m *mongoMapper) ExistsByCodeAndUnitID(ctx context.Context, code string, unitID primitive.ObjectID) (bool, error) {
	return m.ExistsByFields(ctx, bson.M{cst.Code: code, cst.UnitID: unitID})
}

// FindAllByUnitID 根据UnitID查询所有用户
func (m *mongoMapper) FindAllByUnitID(ctx context.Context, unitId primitive.ObjectID) ([]*User, error) {
	return m.FindAllByFields(ctx, bson.M{cst.UnitID: unitId})
}
