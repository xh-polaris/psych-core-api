package service

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/synapse"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/google/wire"
)

var _ IUnitService = (*UnitService)(nil)

type IUnitService interface {
	UnitGetInfo(ctx context.Context, req *core_api.UnitGetInfoReq) (*core_api.UnitGetInfoResp, error)
	UnitUpdateInfo(ctx context.Context, req *core_api.UnitUpdateInfoReq) (*basic.Response, error)
	UnitFindByURI(ctx context.Context, req *core_api.UnitGetByURIReq) (*core_api.UnitGetByURIResp, error)
	UnitCreate(ctx context.Context, req *core_api.CreateUnitReq) (*basic.Response, error)
}

type UnitService struct {
	UnitMapper      unit.IMongoMapper
	UserMapper      user.IMongoMapper
	Synapse4bClient synapse.Client
}

func (u *UnitService) UnitFindByURI(ctx context.Context, req *core_api.UnitGetByURIReq) (*core_api.UnitGetByURIResp, error) {
	un, err := u.UnitMapper.FindOneByURI(ctx, req.Uri)
	if err != nil {
		logs.Errorf("UnitFindByURI failed:%s, got URI:%s", errorx.ErrorWithoutStack(err), req.Uri)
		return nil, errorx.New(errno.ErrUnitFindByURI)
	}

	return &core_api.UnitGetByURIResp{
		Unit: &core_api.UnitVO{Id: un.ID.Hex()},
		Code: 0,
		Msg:  "",
	}, nil
}

var UnitServiceSet = wire.NewSet(
	wire.Struct(new(UnitService), "*"),
	wire.Bind(new(IUnitService), new(*UnitService)),
)

func (u *UnitService) UnitGetInfo(ctx context.Context, req *core_api.UnitGetInfoReq) (*core_api.UnitGetInfoResp, error) {
	// 参数校验
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}

	// 鉴权
	m, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if !m.HasUnitAdminAuth(req.UnitId) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	unitId, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		logs.Errorf("parse unit id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 查询单位
	unitDAO, err := u.UnitMapper.FindOneById(ctx, unitId)
	if err != nil {
		logs.Errorf("find unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 构造返回结果
	return &core_api.UnitGetInfoResp{
		Unit: &core_api.UnitVO{
			Id:         unitDAO.ID.Hex(),
			Name:       unitDAO.Name,
			Address:    unitDAO.Address,
			Contact:    unitDAO.Contact,
			Level:      int32(unitDAO.Level),
			Status:     int32(unitDAO.Status),
			CreateTime: unitDAO.CreateTime.Unix(),
			UpdateTime: unitDAO.UpdateTime.Unix(),
			DeleteTime: unitDAO.DeleteTime.Unix(),
		},
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UnitService) UnitUpdateInfo(ctx context.Context, req *core_api.UnitUpdateInfoReq) (*basic.Response, error) {
	// 参数校验
	if req.Unit.Id == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}

	// 鉴权
	m, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if !m.HasUnitAdminAuth(req.Unit.Id) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 不允许修改手机号、密码、验证方式、level、状态
	// 密码、验证方式需要通过其他接口修改
	unitId, err := bson.ObjectIDFromHex(req.Unit.Id)
	if err != nil {
		logs.Errorf("parse unit id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 构建更新字段
	update := make(bson.M)
	if req.Unit.Name != "" {
		update[cst.Name] = req.Unit.Name
	}
	if req.Unit.Address != "" {
		update[cst.Address] = req.Unit.Address
	}
	if req.Unit.Contact != "" {
		update[cst.Contact] = req.Unit.Contact
	}
	update[cst.UpdateTime] = time.Now().Unix()

	// 一次更新所有字段
	if len(update) > 0 {
		if err = u.UnitMapper.UpdateFields(ctx, unitId, update); err != nil {
			logs.Errorf("update unit error: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}
	}

	// 构造返回结果
	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UnitService) UnitCreate(ctx context.Context, req *core_api.CreateUnitReq) (*basic.Response, error) {
	// 参数校验
	if req.Unit.Name == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位名称"))
	}
	if req.Unit.Address == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位地址"))
	}
	if req.Unit.Contact == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位联系人"))
	}
	if req.Unit.Level == 0 {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位等级"))
	}

	// 鉴权 必须是超管才能创建单位
	m, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if !m.HasUnitAdminAuth(req.Unit.Name) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	adminOid, _ := bson.ObjectIDFromHex(m.UserId)
	superAdmin, err := u.UserMapper.FindOneById(ctx, adminOid)
	if err != nil || superAdmin == nil || superAdmin.Role != enum.UserRoleSuperAdmin {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 创建synapse unit
	sUnit, err := u.Synapse4bClient.CreateUnit(ctx, req.Unit.Name)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrUnitCreate)
	}
	oid, _ := bson.ObjectIDFromHex(sUnit.ID)
	// 创建psych unit
	pUnit := &unit.Unit{
		ID:         oid,
		Name:       req.Unit.Name,
		Address:    req.Unit.Address,
		Contact:    req.Unit.Contact,
		Level:      int(req.Unit.Level),
		Status:     enum.UnitStatusActive,
		URI:        req.Unit.Uri,
		CreateTime: time.Unix(sUnit.CreateTime, 0),
		UpdateTime: time.Unix(sUnit.UpdateTime, 0),
	}

	// 存储psych unit
	if err = u.UnitMapper.Insert(ctx, pUnit); err != nil {
		logs.Errorf("insert unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUnitCreate)
	}

	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}
