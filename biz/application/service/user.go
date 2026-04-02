package service

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/types/enum"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/convert"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/encrypt"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/google/wire"
)

var _ IUserService = (*UserService)(nil)

type IUserService interface {
	UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (*core_api.UserSignInResp, error)
	UserGetInfo(ctx context.Context, req *core_api.UserGetInfoReq) (*core_api.UserGetInfoResp, error)
	UserUpdateInfo(ctx context.Context, req *core_api.UserUpdateInfoReq) (*basic.Response, error)
	UserUpdatePassword(ctx context.Context, req *core_api.UserUpdatePasswordReq) (*basic.Response, error)
}

type UserService struct {
	UserMapper user.IMongoMapper
	UnitMapper unit.IMongoMapper
}

var UserServiceSet = wire.NewSet(
	wire.Struct(new(UserService), "*"),
	wire.Bind(new(IUserService), new(*UserService)),
)

func (u *UserService) UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (*core_api.UserSignInResp, error) {
	// 参数校验
	if req.AuthId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "账号"))
	}
	switch req.AuthType {
	case enum.AuthTypeCode:
		return nil, errorx.New(errno.ErrUnImplement) // TODO: 验证码登录
	case enum.AuthTypePassword: //
		if req.VerifyCode == "" {
			return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "密码"))
		}
	default:
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "登录方式"))
	}

	// 超级管理员不需要单位ID
	var err error
	unitId := ""
	var userDAO *user.User
	switch req.UnitId {
	case "":
		userDAO, err = u.UserMapper.FindOneByCodeAndRole(ctx, req.AuthId, enum.UserRoleSuperAdmin)
		if err != nil {
			logs.Errorf("find user by code %s error: %s", req.AuthId, errorx.ErrorWithoutStack(err))
			return nil, errorx.New(errno.ErrWrongAccountOrPassword)
		}
		if userDAO == nil {
			return nil, errorx.New(errno.ErrWrongAccountOrPassword)
		}
	default:
		uid, err := bson.ObjectIDFromHex(req.UnitId)
		if err != nil {
			logs.Errorf("parse unit id error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("unitId", "单位id"))
		}
		// 获得用户
		userDAO, err = u.UserMapper.FindOneByCodeAndUnitID(ctx, req.AuthId, uid)
		if err != nil {
			logs.Errorf("find user by code %s and unit id %s error: %s", req.AuthId, uid, errorx.ErrorWithoutStack(err))
			return nil, errorx.New(errno.ErrUserNotFound)
		}
		unitId = userDAO.UnitID.Hex()
	}

	// 密码验证
	if isValid := encrypt.BcryptCheck(req.VerifyCode, userDAO.Password); !isValid {
		return nil, errorx.New(errno.ErrWrongAccountOrPassword)
	}

	// 签发jwt
	token, err := util.GenerateJwt(map[string]any{
		cst.JsonUnitID: unitId,
		cst.JsonUserID: userDAO.ID.Hex(),
		cst.JsonCode:   userDAO.Code, // 手机号或学号
		cst.JsonRole:   userDAO.Role,
	})
	if err != nil {
		logs.Errorf("generate token for UserSignIn error: %s", errorx.ErrorWithoutStack(err))
	}

	return &core_api.UserSignInResp{
		UnitId:    userDAO.UnitID.Hex(), // 超级管理员单位ID为""
		UserId:    userDAO.ID.Hex(),
		CodeValue: userDAO.Code,
		CodeType:  int32(userDAO.CodeType),
		Token:     token,
		Code:      0,
		Msg:       "success",
	}, nil
}

func (u *UserService) UserGetInfo(ctx context.Context, req *core_api.UserGetInfoReq) (*core_api.UserGetInfoResp, error) {
	// 参数校验
	if req.UserId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "用户ID"))
	}

	// 转换用户ID
	userId, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		logs.Errorf("parse user id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "用户ID"))
	}

	// 获得用户
	userDAO, err := u.UserMapper.FindOneById(ctx, userId)
	if err != nil {
		logs.Errorf("find user error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	optionsAny, err := convert.Any2Anypb(userDAO.Options)
	if err != nil {
		return nil, err
	}

	return &core_api.UserGetInfoResp{
		User: &core_api.UserVO{
			Id:         userDAO.ID.Hex(),
			CodeType:   int32(userDAO.CodeType),
			Code:       userDAO.Code,
			UnitId:     userDAO.UnitID.Hex(),
			Name:       userDAO.Name,
			Gender:     int32(userDAO.Gender),
			Birth:      userDAO.Birth.Unix(),
			Status:     int32(userDAO.Status),
			EnrollYear: int32(userDAO.EnrollYear),
			Class:      int32(userDAO.Class),
			Grade:      int32(userDAO.Grade),
			Role:       int32(userDAO.Role),
			Options:    optionsAny,
			CreateTime: userDAO.CreateTime.Unix(),
			UpdateTime: userDAO.UpdateTime.Unix(),
			DeleteTime: userDAO.DeleteTime.Unix(),
		},
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UserService) UserUpdateInfo(ctx context.Context, req *core_api.UserUpdateInfoReq) (*basic.Response, error) {
	// 参数校验
	if req.User.Id == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "用户ID"))
	}

	// 不允许修改手机号、密码、验证方式、单位ID、状态、Remark
	// 密码、验证方式需要通过其他接口修改
	userId, err := bson.ObjectIDFromHex(req.User.Id)
	if err != nil {
		logs.Errorf("parse user id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 构建更新字段
	update := make(bson.M)
	if req.User.Name != "" {
		update[cst.Name] = req.User.Name
	}
	if req.User.Gender != 0 {
		update[cst.Gender] = int(req.User.Gender)
	}
	if req.User.Birth != 0 {
		update[cst.Birth] = time.Unix(req.User.Birth, 0)
	}
	if req.User.EnrollYear != 0 {
		update[cst.EnrollYear] = int(req.User.EnrollYear)
	}
	if req.User.Class != 0 {
		update[cst.Class] = int(req.User.Class)
	}
	if req.User.Grade != 0 {
		update[cst.Grade] = int(req.User.Grade)
	}
	if req.User.Options != nil {
		optionsAnypb, err := convert.Anypb2Any(req.User.Options)
		if err != nil {
			return nil, err
		}
		update[cst.Options] = optionsAnypb
	}
	update[cst.UpdateTime] = time.Now().Unix()

	// 一次更新所有字段
	if len(update) > 0 {
		if err = u.UserMapper.UpdateFields(ctx, userId, update); err != nil {
			logs.Errorf("update user error: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}
	}

	// 构造返回结果
	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UserService) UserUpdatePassword(ctx context.Context, req *core_api.UserUpdatePasswordReq) (*basic.Response, error) {
	// 参数校验
	if req.Id == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	//if req.AuthType == "" {
	//	return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "验证方式"))
	//}
	if req.VerifyCode == "" && req.AuthType == enum.AuthTypePassword {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "旧密码"))
	}
	if req.VerifyCode == "" && req.AuthType == enum.AuthTypeCode {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "验证码"))
	}
	if req.NewPassword == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "新密码"))
	}

	userId, err := bson.ObjectIDFromHex(req.Id)
	if err != nil {
		logs.Errorf("parse user id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 验证方式
	userDAO := &user.User{}
	switch req.AuthType {
	// 验证码
	case enum.AuthTypeCode:
		return nil, errorx.New(errno.ErrUnImplement) // TODO: 验证码登录
	// 密码
	case enum.AuthTypePassword:
		// 获取密码
		userDAO, err = u.UserMapper.FindOneById(ctx, userId)
		if err != nil {
			logs.Errorf("find user by phone error: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}
		if !encrypt.BcryptCheck(req.VerifyCode, userDAO.Password) {
			return nil, errorx.New(errno.ErrWrongPassword)
		}
	}

	// 加密密码
	newPwd, err := encrypt.BcryptEncrypt(req.NewPassword)
	if err != nil {
		logs.Errorf("bcrypt encrypt error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 更新密码
	if err = u.UserMapper.UpdateFields(ctx, userDAO.ID, bson.M{
		cst.Password:   newPwd,
		cst.UpdateTime: time.Now().Unix(),
	}); err != nil {
		logs.Errorf("update user error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 构造返回结果
	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}
