package service

import (
	"context"
	"errors"
	"strings"
	"time"

	appdto "github.com/xh-polaris/psych-core-api/biz/application/dto"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/domain/usr"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/convert"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/google/wire"
)

var _ IUserService = (*UserService)(nil)

type IUserService interface {
	UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (*core_api.UserSignInResp, error) // Deprecated
	UserGetInfo(ctx context.Context, req *core_api.UserGetInfoReq) (*core_api.UserGetInfoResp, error)
	UserUpdateInfo(ctx context.Context, req *core_api.UserUpdateInfoReq) (*basic.Response, error)
	UserUpdatePassword(ctx context.Context, req *core_api.UserUpdatePasswordReq) (*basic.Response, error)
	CreateUser(ctx context.Context, req *core_api.CreateUserReq) (*core_api.CreateUserResp, error)
	SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*basic.Response, error)
}

type UserService struct {
	UserDomain usr.IUserDomainSVC
	UserMapper user.IMongoMapper
	UnitMapper unit.IMongoMapper
	AuthDomain auth.IAuthDomain
}

var UserServiceSet = wire.NewSet(
	wire.Struct(new(UserService), "*"),
	wire.Bind(new(IUserService), new(*UserService)),
)

// UserSignIn .
func (u *UserService) UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (*core_api.UserSignInResp, error) {
	// 参数校验
	if req.AuthId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "身份凭证"))
	}
	if req.VerifyCode == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "验证码或密码"))
	}

	var psychUser *user.User
	var err error
	switch {
	case strings.HasPrefix(req.AuthType, "phone-"):
		psychUser, err = u.UserDomain.SignInByPhone(ctx, req.AuthType, req.AuthId, req.GetUnitId(), req.VerifyCode)
	case strings.HasPrefix(req.AuthType, "email-"):
		psychUser, err = u.UserDomain.SignInByEmail(ctx, req.AuthType, req.AuthId, req.GetUnitId(), req.VerifyCode)
	case strings.HasPrefix(req.AuthType, "code-"):
		psychUser, err = u.UserDomain.SignInByCode(ctx, req.AuthType, req.AuthId, req.GetUnitId(), req.VerifyCode)
	default:
		return nil, errorx.New(errno.ErrUnSupportAuthType)
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errorx.New(errno.ErrSignIn, errorx.KV("field", "用户不存在"))
	} else if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSignIn, errorx.KV("field", err.Error()))
	}

	// 签发 JWT
	token, err := util.GenerateJwt(map[string]any{
		cst.JsonUnitID: psychUser.UnitID.Hex(),
		cst.JsonUserID: psychUser.ID.Hex(),
		cst.JsonCode:   psychUser.Code,
		cst.JsonRole:   psychUser.Role,
	})
	if err != nil {
		logs.Errorf("[StudentSignIn] generate token error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrSignIn, errorx.KV("field", "登录token签发失败"))
	}

	return &core_api.UserSignInResp{
		UnitId:    psychUser.UnitID.Hex(),
		UserId:    psychUser.ID.Hex(),
		Token:     token,
		CodeValue: psychUser.Code,
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

	unitDAO, err := u.UnitMapper.FindOneById(ctx, userDAO.UnitID)
	if err != nil {
		logs.Errorf("find unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	calculatedGrade := userDAO.CalculateGrade(unitDAO.StartGrade)

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
			Grade:      int32(calculatedGrade),
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

func (u *UserService) GetStudentProfile(ctx context.Context) (*appdto.StudentProfile, error) {
	meta, err := u.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := bson.ObjectIDFromHex(meta.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}
	student, err := u.UserMapper.FindOneById(ctx, userID)
	if err != nil {
		return nil, err
	}
	unitDAO, err := u.UnitMapper.FindOneById(ctx, student.UnitID)
	if err != nil {
		return nil, err
	}
	birth := int64(0)
	if !student.Birth.IsZero() {
		birth = student.Birth.Unix()
	}
	avatar := strings.TrimSpace(student.Avatar)
	if avatar != "" && !strings.HasPrefix(avatar, "http://") && !strings.HasPrefix(avatar, "https://") {
		avatar = "https://" + strings.TrimLeft(avatar, "/")
	}
	return &appdto.StudentProfile{
		ID: student.ID.Hex(), Code: student.Code, Name: student.Name,
		Avatar: avatar, Birth: birth, Gender: student.Gender,
		EnrollYear: student.EnrollYear, Grade: student.CalculateGrade(unitDAO.StartGrade),
		Class: student.Class,
	}, nil
}

func (u *UserService) UpdateStudentProfile(ctx context.Context, input *appdto.UpdateStudentProfileInput) (*appdto.StudentProfile, error) {
	meta, err := u.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := bson.ObjectIDFromHex(meta.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "姓名"))
	}
	if input.Gender < 0 || input.Gender > 3 {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "性别"))
	}
	update := bson.M{
		cst.Name: name, "avatar": strings.TrimSpace(input.Avatar),
		cst.Gender: input.Gender, cst.UpdateTime: time.Now(),
	}
	if input.Birth > 0 {
		update[cst.Birth] = time.Unix(input.Birth, 0)
	}
	if err = u.UserMapper.UpdateFields(ctx, userID, update); err != nil {
		return nil, err
	}
	return u.GetStudentProfile(ctx)
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
	if req.User.Options != nil {
		optionsAnypb, err := convert.Anypb2Any(req.User.Options)
		if err != nil {
			return nil, err
		}
		update[cst.Options] = optionsAnypb
	}
	update[cst.UpdateTime] = time.Now()

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
	if req.NewPassword == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "新密码"))
	}
	// 检查是否是登录态修改密码
	um, err := u.AuthDomain.ExtraUserMeta(ctx)
	if err != nil && um != nil {
		// 调用user域更新密码
		err = u.UserDomain.UpdatePassword(ctx, um.UserId, req.NewPassword)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrUpdatePassword)
		}

		return &basic.Response{
			Code: 0,
			Msg:  "success",
		}, nil
	}

	// 非登录态修改密码 需要目标userId，验证凭证等参数
	if req.UserId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "用户ID"))
	}
	if req.AuthId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "验证凭证(手机号/邮箱/学号)"))
	}
	if req.VerifyCode == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "旧密码/验证码"))
	}
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	oids, err := util.ObjectIDsFromHex(req.UserId, req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}

	if err := u.UserDomain.ValidatePasswordResetTarget(ctx, oids[1].Hex()); err != nil {
		return nil, err
	}

	// 调用user域更新密码
	err = u.UserDomain.UpdatePassword(ctx, req.UserId, req.NewPassword)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrUpdatePassword)
	}

	// 构造返回结果
	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UserService) CreateUser(ctx context.Context, req *core_api.CreateUserReq) (*core_api.CreateUserResp, error) {
	input := usr.CreateUserInput{
		UnitID:     req.UnitId,
		Password:   req.Password,
		Email:      req.Email,
		Phone:      req.Phone,
		Code:       req.Code,
		CodeType:   int(req.CodeType),
		Name:       req.Name,
		Birth:      req.Birth,
		Gender:     int(req.Gender),
		EnrollYear: int(req.EnrollYear),
		Role:       int(req.Role),
		Class:      int(req.Class),
		CreateTime: req.CreateTime,
		UpdateTime: req.UpdateTime,
	}
	if err := u.UserDomain.ValidateCreateUserInput(input); err != nil {
		return nil, err
	}

	// 权限校验-需要超管权限
	operator, err := u.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if err := u.AuthDomain.VerifySuperAdmin(ctx, operator); err != nil {
		return nil, err
	}

	// 账号标识校验、单位双端一致性校验，以及中台账号与本地用户的创建，
	// 都属于 UserDomain 的账号一致性规则。
	puWithId, pUnit, err := u.UserDomain.CreateUser(ctx, input)
	if err != nil {
		return nil, err
	}

	calculatedGrade := puWithId.CalculateGrade(pUnit.StartGrade)

	// 构建响应
	ru := &core_api.UserVO{
		Id:         puWithId.ID.Hex(),
		Name:       puWithId.Name,
		Gender:     int32(puWithId.Gender),
		Role:       int32(puWithId.Role),
		EnrollYear: int32(puWithId.EnrollYear),
		Grade:      int32(calculatedGrade),
		Class:      int32(puWithId.Class),
		UnitId:     req.UnitId,
		Code:       puWithId.Code,
		CodeType:   int32(puWithId.CodeType),
		CreateTime: puWithId.CreateTime.Unix(),
	}

	return &core_api.CreateUserResp{
		User: ru,
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UserService) SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*basic.Response, error) {
	if err := u.UserDomain.SendVerifyCode(ctx, req.AuthType, req.AuthId, ""); err != nil {
		return nil, err
	}

	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}
