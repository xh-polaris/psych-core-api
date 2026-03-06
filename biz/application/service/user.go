package service

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/convert"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/encrypt"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/enum"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/reg"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-idl/kitex_gen/basic"
)

var _ IUserService = (*UserService)(nil)

type IUserService interface {
	UserSignUp(ctx context.Context, req *core_api.UserSignUpReq) (*core_api.UserSignUpResp, error)
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

// UserSignUp 用户不能直接注册 即使注册也需要绑定unitId
func (u *UserService) UserSignUp(ctx context.Context, req *core_api.UserSignUpReq) (*core_api.UserSignUpResp, error) {
	// 默认用户通过注册接口，使用手机号注册
	// 参数校验
	if req.User == nil {
		return nil, errorx.New(errno.ErrMissingEntity, errorx.KV("entity", "用户"))
	}
	if req.User.Code == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "电话号码"))
	}
	if req.User.Password == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "密码"))
	}
	if req.User.Name == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "姓名"))
	}

	// 手机号格式校验
	if !reg.CheckMobile(req.User.Code) {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "电话号码"))
	}

	// 检查手机号是否已注册
	if exists, err := u.UserMapper.ExistsByCode(ctx, req.User.Code); err != nil {
		logs.Errorf("check phone exists error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	} else if exists {
		return nil, errorx.New(errno.ErrPhoneAlreadyExist)
	}

	// 密码加密
	hashedPwd, err := encrypt.BcryptEncrypt(req.User.Password)
	if err != nil {
		logs.Errorf("bcrypt encrypt error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 转换枚举值
	gender, ok := enum.ParseGender(req.User.Gender)
	if !ok {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "性别"))
	}

	// 转换ID
	var unitId bson.ObjectID
	if req.User.UnitId != "" {
		unitId, err = bson.ObjectIDFromHex(req.User.UnitId)
		if err != nil {
			logs.Errorf("parse unit id error: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}
	}

	// 构造用户
	userDAO := &user.User{
		ID:         bson.NewObjectID(),
		CodeType:   enum.CodeTypePhone,
		Code:       req.User.Code,
		Password:   hashedPwd,
		Name:       req.User.Name,
		Birth:      time.Unix(req.User.Birth, 0),
		Gender:     gender,
		Status:     enum.Active,
		Class:      req.User.Class,
		Grade:      req.User.Grade,
		EnrollYear: req.User.EnrollYear,
		UnitID:     unitId,
		UpdateTime: time.Now(),
		CreateTime: time.Now(),
	}

	// 插入用户
	if err = u.UserMapper.Insert(ctx, userDAO); err != nil {
		logs.Errorf("insert user error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 获得枚举值
	genderStr, ok := enum.GetGender(userDAO.Gender)
	if !ok {
		return nil, errorx.New(errno.ErrInternalError)
	}
	statusStr, ok := enum.GetStatus(userDAO.Status)
	if !ok {
		return nil, errorx.New(errno.ErrInternalError)
	}
	codeTypeStr, ok := enum.GetCodeType(userDAO.CodeType)
	if !ok {
		return nil, errorx.New(errno.ErrInternalError)
	}

	// 构造响应
	return &core_api.UserSignUpResp{
		User: &core_api.User{
			Id:         userDAO.ID.Hex(),
			CodeType:   codeTypeStr,
			Code:       userDAO.Code,
			Name:       userDAO.Name,
			Gender:     genderStr,
			Birth:      userDAO.Birth.Unix(),
			Class:      userDAO.Class,
			Grade:      userDAO.Grade,
			EnrollYear: userDAO.EnrollYear,
			Status:     statusStr,
			CreateTime: userDAO.CreateTime.Unix(),
			UpdateTime: userDAO.UpdateTime.Unix(),
		},
		Code: 200,
		Msg:  "success",
	}, nil
}

func (u *UserService) UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (*core_api.UserSignInResp, error) {
	// 参数校验
	//if req.AuthType == "" {
	//	return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "登录方式"))
	//}
	if req.AuthId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "账号"))
	}
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	switch req.AuthType {
	case cst.AuthTypeCode:
		return nil, errorx.New(errno.ErrUnImplement) // TODO: 验证码登录
	case cst.AuthTypePassword:
		if req.VerifyCode == "" {
			return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "密码"))
		}
	default:
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "登录方式"))
	}

	unitId, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		logs.Errorf("parse unit id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 获得用户
	userDAO, err := u.UserMapper.FindOneByCodeAndUnitID(ctx, req.AuthId, unitId)
	if err != nil {
		logs.Errorf("find user by code and unit id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	if userDAO == nil {
		return nil, errorx.New(errno.ErrWrongAccountOrPassword)
	}

	// 密码验证
	if !encrypt.BcryptCheck(req.VerifyCode, userDAO.Password) {
		return nil, errorx.New(errno.ErrWrongAccountOrPassword)
	}
	codeType, _ := enum.GetCodeType(userDAO.CodeType)
	return &core_api.UserSignInResp{
		UnitId:    userDAO.UnitID.Hex(),
		UserId:    userDAO.ID.Hex(),
		CodeValue: userDAO.Code,
		CodeType:  codeType,
		Code:      200,
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

	// 获得枚举值
	genderStr, ok := enum.GetGender(userDAO.Gender)
	if !ok {
		return nil, errorx.New(errno.ErrInternalError)
	}
	statusStr, ok := enum.GetStatus(userDAO.Status)
	if !ok {
		return nil, errorx.New(errno.ErrInternalError)
	}
	codeTypeStr, ok := enum.GetCodeType(userDAO.CodeType)
	if !ok {
		return nil, errorx.New(errno.ErrInternalError)
	}

	optionsAny, err := convert.Any2Anypb(userDAO.Options)
	if err != nil {
		return nil, err
	}

	return &core_api.UserGetInfoResp{
		User: &core_api.User{
			Id:         userDAO.ID.Hex(),
			CodeType:   codeTypeStr,
			Code:       userDAO.Code,
			UnitId:     userDAO.UnitID.Hex(),
			Name:       userDAO.Name,
			Gender:     genderStr,
			Birth:      userDAO.Birth.Unix(),
			Status:     statusStr,
			EnrollYear: userDAO.EnrollYear,
			Class:      userDAO.Class,
			Grade:      userDAO.Grade,
			Options:    optionsAny,
			CreateTime: userDAO.CreateTime.Unix(),
			UpdateTime: userDAO.UpdateTime.Unix(),
			DeleteTime: userDAO.DeleteTime.Unix(),
		},
		Code: 200,
		Msg:  "success",
	}, nil
}

func (u *UserService) UserUpdateInfo(ctx context.Context, req *core_api.UserUpdateInfoReq) (*basic.Response, error) {
	// 参数校验
	if req.User.Id == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "用户ID"))
	}

	// 不允许修改手机号、密码、验证方式、单位ID、状态
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
	if req.User.Gender != "" {
		gender, ok := enum.ParseGender(req.User.Gender)
		if !ok {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "性别"))
		}
		update[cst.Gender] = gender
	}
	if req.User.Birth != 0 {
		update[cst.Birth] = req.User.Birth
	}
	if req.User.EnrollYear != 0 {
		update[cst.EnrollYear] = req.User.EnrollYear
	}
	if req.User.Class != 0 {
		update[cst.Class] = req.User.Class
	}
	if req.User.Grade != 0 {
		update[cst.Grade] = req.User.Grade
	}
	if req.User.Options != nil {
		optionsAnypb, err := convert.Anypb2Any(req.User.Options)
		if err != nil {
			return nil, err
		}
		update[cst.Options] = optionsAnypb
	}
	if req.User.Remark != nil {
		update[cst.Remark] = req.User.Remark
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
		Code: 200,
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
	if req.VerifyCode == "" && req.AuthType == cst.AuthTypePassword {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "旧密码"))
	}
	if req.VerifyCode == "" && req.AuthType == cst.AuthTypeCode {
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
	case cst.AuthTypeCode:
		return nil, errorx.New(errno.ErrUnImplement) // TODO: 验证码登录
	// 密码
	case cst.AuthTypePassword:
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
		Code: 200,
		Msg:  "success",
	}, nil
}
