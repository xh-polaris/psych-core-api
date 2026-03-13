package service

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/encrypt"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/enum"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/reg"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/google/wire"
)

var _ IUnitService = (*UnitService)(nil)

type IUnitService interface {
	UnitGetInfo(ctx context.Context, req *core_api.UnitGetInfoReq) (*core_api.UnitGetInfoResp, error)
	UnitUpdateInfo(ctx context.Context, req *core_api.UnitUpdateInfoReq) (*basic.Response, error)
	UnitLinkUser(ctx context.Context, req *core_api.UnitLinkUserReq) (*basic.Response, error)
	UnitCreateAndLinkUser(ctx context.Context, req *core_api.UnitCreateAndLinkUserReq) (*core_api.UnitCreateAndLinkUserResp, error)
}

type UnitService struct {
	UnitMapper unit.IMongoMapper
	UserMapper user.IMongoMapper
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

	// 获得单位状态
	statusStr, ok := enum.GetStatus(unitDAO.Status)
	if !ok {
		return nil, errorx.New(errno.ErrInternalError)
	}

	// 构造返回结果
	return &core_api.UnitGetInfoResp{
		Unit: &core_api.UnitVO{
			Id:         unitDAO.ID.Hex(),
			Name:       unitDAO.Name,
			Address:    unitDAO.Address,
			Contact:    unitDAO.Contact,
			Level:      int32(unitDAO.Level),
			Status:     statusStr,
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

func (u *UnitService) UnitLinkUser(ctx context.Context, req *core_api.UnitLinkUserReq) (*basic.Response, error) {
	// 参数校验
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	if req.UserId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "用户ID"))
	}

	// 转换ID
	unitId, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		logs.Errorf("parse unit id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	userId, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		logs.Errorf("parse user id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 绑定用户
	if err := u.UserMapper.UpdateFields(ctx, userId, bson.M{cst.UnitID: unitId}); err != nil {
		logs.Errorf("update user error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UnitService) UnitCreateAndLinkUser(ctx context.Context, req *core_api.UnitCreateAndLinkUserReq) (*core_api.UnitCreateAndLinkUserResp, error) {
	// 参数校验
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	if req.CodeType == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "验证方式"))
	}
	if len(req.Users) == 0 {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "用户列表"))
	}

	// 提取枚举值
	codeType, ok := enum.ParseCodeType(req.CodeType)
	if !ok {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "验证方式"))
	}

	// 转换ID
	unitId, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		logs.Errorf("parse unit id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 验证方式标记
	isCodeTypePhone := codeType == enum.CodeTypePhone

	// 找出所有属于这个单位的用户
	users, err := u.UserMapper.FindAllByUnitID(ctx, unitId)
	if err != nil {
		logs.Errorf("find users by unit id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 创建一个map用于快速查找已存在的用户code
	existingCodes := make(map[string]bool)
	for _, userDAO := range users {
		existingCodes[userDAO.Code] = true
	}

	// 记录需要插入的用户数量、成功数量和跳过数量
	all := len(req.Users)
	success := 0
	skip := 0

	// 插入用户
	for _, userReq := range req.Users {
		// 参数校验
		if userReq.Code == "" && isCodeTypePhone {
			return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "电话"))
		}
		if userReq.Code == "" && !isCodeTypePhone {
			return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "学号"))
		}
		if userReq.Name == "" {
			return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "姓名"))
		}
		if userReq.Password == "" {
			return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "密码"))
		}

		// 检查是否已存在相同的code
		if existingCodes[userReq.Code] {
			// 如果在这个unit中已经存在该code，则跳过
			skip++
			continue
		}

		if isCodeTypePhone {
			// 检查同一Unit下手机号是否已注册
			if exists, err := u.UserMapper.ExistsByCodeAndUnitID(ctx, userReq.Code, unitId); err != nil {
				logs.Errorf("check phone exists in unit error: %s", errorx.ErrorWithoutStack(err))
				return nil, err
			} else if exists {
				// 如果在这个unit中已经存在该手机号，则跳过
				skip++
				continue
			}

			// 如果说验证方式是手机，则需要检测手机号的格式
			if !reg.CheckMobile(userReq.Code) {
				return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "手机号"))
			}
		} else {
			// 检查同一Unit下学号是否已注册
			if exists, err := u.UserMapper.ExistsByCodeAndUnitID(ctx, userReq.Code, unitId); err != nil {
				logs.Errorf("check student id exists in unit error: %s", errorx.ErrorWithoutStack(err))
				return nil, err
			} else if exists {
				// 如果在这个unit中已经存在该学号，则跳过
				skip++
				continue
			}
		}

		// 加密密码
		hashedPwd, err := encrypt.BcryptEncrypt(userReq.Password)
		if err != nil {
			logs.Errorf("bcrypt encrypt error: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}

		// 提取枚举值
		gender, ok := enum.ParseGender(userReq.Gender)
		if !ok {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "性别"))
		}
		codeType, ok := enum.ParseCodeType(req.CodeType)
		if !ok {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "验证方式"))
		}

		// 构造用户
		userDAO := &user.User{
			ID:         bson.NewObjectID(),
			CodeType:   codeType,
			Code:       userReq.Code,
			Password:   hashedPwd,
			Name:       userReq.Name,
			Birth:      time.Unix(userReq.Birth, 0),
			Gender:     gender,
			Status:     enum.Active,
			Class:      int(userReq.Class),
			Grade:      int(userReq.Grade),
			EnrollYear: int(userReq.EnrollYear),
			UnitID:     unitId,
			UpdateTime: time.Now(),
			CreateTime: time.Now(),
		}

		// 插入用户
		if err = u.UserMapper.Insert(ctx, userDAO); err != nil {
			logs.Errorf("insert user error: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}

		// 添加到existingCodes map中，避免后续重复创建
		existingCodes[userReq.Code] = true

		// 添加成功数量
		success++
	}

	return &core_api.UnitCreateAndLinkUserResp{
		AllCount:     int32(all),
		SuccessCount: int32(success),
		SkipCount:    int32(skip),
		Code:         0,
		Msg:          "success",
	}, nil
}
