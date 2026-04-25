package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/domain/usr"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/synapse"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/encrypt"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util/convert"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
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
	SuperAdminSignIn(ctx context.Context, req *core_api.UserSignInReq) (*core_api.UserSignInResp, error)
}

type UserService struct {
	UserDomain usr.IUserDomainSVC
	UserMapper user.IMongoMapper
	UnitMapper unit.IMongoMapper
	Synp4bCli  synapse.Client
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
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSignIn)
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
		return nil, errorx.WrapByCode(err, errno.ErrSignIn)
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
	if req.NewPassword == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "新密码"))
	}
	// 检查是否是登录态修改密码
	um, err := util.ExtraUserMeta(ctx)
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

	// 校验unit存在
	// synapse unit
	su, err := u.Synp4bCli.GetUnit(ctx, req.UnitId)
	if err != nil || su == nil {
		return nil, errorx.WrapByCode(err, errno.ErrCreateUser)
	}
	// psych unit
	pUnit, err := u.UnitMapper.FindOneById(ctx, oids[1])
	if pUnit == nil || errors.Is(err, mongo.ErrNoDocuments) || pUnit.Status != enum.UnitStatusActive {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", fmt.Sprintf("指定单位[id=%s]", req.UnitId)))
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
	// basic user参数校验
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	unitOid, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}

	if req.Code == nil && req.Phone == nil && req.Email == nil {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "验证码/手机号/邮箱不能全为空"))
	}

	if req.Password == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "密码"))
	}

	// 权限校验-需要超管权限
	operator, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	oid, _ := bson.ObjectIDFromHex(operator.UserId)
	operatorUser, err := u.UserMapper.FindOneById(ctx, oid)
	if err != nil {
		logs.Error("[user mapper] FindOneById failed")
		return nil, errorx.WrapByCode(err, errno.ErrInternalError)
	}

	if !operator.HasSuperAdminAuth() || operatorUser.Role != enum.UserRoleSuperAdmin {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 校验unit存在
	// synapse unit
	su, err := u.Synp4bCli.GetUnit(ctx, req.UnitId)
	if err != nil || su == nil {
		return nil, errorx.WrapByCode(err, errno.ErrCreateUser)
	}
	// psych unit
	pUnit, err := u.UnitMapper.FindOneById(ctx, unitOid)
	if pUnit == nil || errors.Is(err, mongo.ErrNoDocuments) || pUnit.Status != enum.UnitStatusActive {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", fmt.Sprintf("指定单位[id=%s]", req.UnitId)))
	} else if err != nil {
		logs.Errorf("find unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}

	// 校验psychUser字段
	pu, err := tryBuildPsychUser(req, pUnit)
	if err != nil {
		return nil, err
	}

	// 调用domain层创建用户
	puWithId, err := u.UserDomain.CreateUser(ctx, req.UnitId, req.GetEmail(), req.GetPhone(), req.GetCode(), req.Password, pu)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrCreateUser)
	}

	unitDAO, err := u.UnitMapper.FindOneById(ctx, puWithId.UnitID)
	if err != nil {
		logs.Errorf("find unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	calculatedGrade := puWithId.CalculateGrade(unitDAO.StartGrade)

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

func tryBuildPsychUser(req *core_api.CreateUserReq, pUnit *unit.Unit) (*user.User, error) {
	// psychUser所需参数校验
	if req.Name == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "姓名"))
	}
	if req.EnrollYear == 0 {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "入学年份"))
	}
	if req.Class == 0 {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "班级"))
	}
	if req.CodeType == 0 {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "code类型"))
	}
	if req.Role == 0 {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "角色"))
	}
	if req.Role > enum.UserRoleUnitAdmin {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "用户角色不合法"))
	}

	if req.Gender != enum.UserGenderMale && req.Gender != enum.UserGenderFemale {
		req.Gender = enum.UserGenderOther
	}

	// 处理 UnitId
	unitOid, _ := bson.ObjectIDFromHex(req.UnitId)

	// 同时传入phone, email, code时，psych user存储优先级phone > email > studentID
	var code string
	var codeType int
	if req.GetCode() != "" {
		code = req.GetCode()
		codeType = enum.UserCodeTypeStudentID
	}
	if req.GetEmail() != "" {
		code = req.GetEmail()
		codeType = enum.UserCodeTypeEmail
	}
	if req.GetPhone() != "" {
		code = req.GetPhone()
		codeType = enum.UserCodeTypePhone
	}

	// birth
	var birth time.Time
	if req.Birth != 0 {
		birth = time.Unix(req.Birth, 0)
	}

	now := time.Now()

	// 角色
	if req.Role == 0 {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "角色"))
	}
	if req.Role != enum.UserRoleStudent && req.Role != enum.UserRoleTeacher && req.Role != enum.UserRoleClassTeacher && req.Role != enum.UserRoleUnitAdmin {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "用户角色不合法"))
	}

	// 时间字段: CreateTime, UpdateTime
	var createTime time.Time
	var updateTime time.Time
	if req.CreateTime != 0 {
		createTime = time.Unix(req.CreateTime, 0)
	} else {
		createTime = now
	}
	if req.UpdateTime != 0 {
		updateTime = time.Unix(req.UpdateTime, 0)
	} else {
		updateTime = now
	}

	// 构造 user 对象
	u := &user.User{
		CodeType:   codeType,
		Code:       code,
		UnitID:     unitOid,
		Name:       req.Name,
		Birth:      birth,
		Gender:     int(req.Gender),
		RiskLevel:  enum.UserRiskLevelNormal,
		Status:     enum.UserStatusActive,
		EnrollYear: int(req.EnrollYear),
		Role:       int(req.Role),
		Class:      int(req.Class),
		CreateTime: createTime,
		UpdateTime: updateTime,
	}

	return u, nil
}

func (u *UserService) SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*basic.Response, error) {
	// 校验authId 确保是既有账号
	ok, err := u.UserMapper.ExistsByCode(ctx, req.AuthId)
	if !ok {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", fmt.Sprintf("指定用户[code=%s]", req.AuthId)))
	} else if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSendVerifyCode)
	}

	err = u.UserDomain.SendVerifyCode(ctx, req.AuthType, req.AuthId, "")
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSendVerifyCode)
	}

	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (u *UserService) SuperAdminSignIn(ctx context.Context, req *core_api.UserSignInReq) (*core_api.UserSignInResp, error) {
	// 参数校验
	if req.AuthId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "身份凭证"))
	}
	if req.VerifyCode == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "密码/验证码"))
	}

	// 超管无需走synapse4b中台
	superAdmin, err := u.UserMapper.FindOneByCodeAndRole(ctx, req.AuthId, enum.UserRoleSuperAdmin)
	if errors.Is(err, mongo.ErrNoDocuments) || superAdmin == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", fmt.Sprintf("超管用户[code=%s]", req.AuthId)))
	} else if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSignIn)
	}

	// 校验密码/验证码
	switch req.AuthType {
	case cst.AuthTypePhoneVerify:
		//err = u.Synp4bCli.CheckVerifyCode(ctx, req.AuthType, req.AuthId, "", req.VerifyCode)
		//if err != nil {
		//	return nil, errorx.New(errno.ErrWrongPassword)
		//}
		return nil, errorx.New(errno.ErrUnsupported)
	case cst.AuthTypePhonePassword:
		// 超管用name字段存bcrypt加密的密码
		if ok := encrypt.BcryptCheck(req.VerifyCode, superAdmin.Name); !ok {
			return nil, errorx.New(errno.ErrWrongPassword)
		}

	default:
		return nil, errorx.New(errno.ErrUnSupportAuthType)
	}

	// 签发 JWT
	token, err := util.GenerateJwt(map[string]any{
		cst.JsonUnitID: "",
		cst.JsonUserID: superAdmin.ID.Hex(),
		cst.JsonCode:   superAdmin.Code,
		cst.JsonRole:   enum.UserRoleSuperAdmin,
	})
	if err != nil {
		logs.Errorf("[SuperAdminSignIn] generate token error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrSignIn)
	}

	return &core_api.UserSignInResp{
		UserId:    superAdmin.ID.Hex(),
		Token:     token,
		CodeValue: superAdmin.Code,
		Code:      0,
		Msg:       "success",
	}, nil
}
