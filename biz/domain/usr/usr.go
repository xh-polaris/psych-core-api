package usr

import (
	"context"
	"errors"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/synapse"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ IUserDomainSVC = (*UserDomainSVC)(nil)

type IUserDomainSVC interface {
	SignInByPhone(ctx context.Context, authType, phone, unitId, verify string) (*user.User, error)
	SignInByEmail(ctx context.Context, authType, email, unitId, verify string) (*user.User, error)
	SignInByCode(ctx context.Context, authType, studentID, unitId, verify string) (*user.User, error)
	CreateUser(ctx context.Context, unitId, email, phone, code, password string, psychUser *user.User) (*user.User, error)
	UpdatePassword(ctx context.Context, userId, newPassword string) error
	SendVerifyCode(ctx context.Context, authType, authId, cause string) error
}

type UserDomainSVC struct {
	UsrMapper  user.IMongoMapper
	UnitMapper unit.IMongoMapper
	Synp4bCli  synapse.Client
}

var UserDomainSet = wire.NewSet(
	wire.Struct(new(UserDomainSVC), "*"),
	wire.Bind(new(IUserDomainSVC), new(*UserDomainSVC)),
)

func (u *UserDomainSVC) SignInByPhone(ctx context.Context, authType, phone, unitId, verify string) (*user.User, error) {
	// 验证中台账号
	synpResp, err := u.Synp4bCli.Login(ctx, authType, phone, unitId, verify)
	if err != nil {
		return nil, err
	}

	// 查询本地账号
	pu, err := u.findPsychUser(ctx, synpResp)
	if err != nil || pu == nil {
		return nil, err
	}

	return pu, nil
}

func (u *UserDomainSVC) SignInByEmail(ctx context.Context, authType, email, unitId, verify string) (*user.User, error) {
	// 验证中台账号
	synpResp, err := u.Synp4bCli.Login(ctx, authType, email, unitId, verify)
	if err != nil {
		return nil, err
	}

	// 查询本地账号
	pu, err := u.findPsychUser(ctx, synpResp)
	if err != nil || pu == nil {
		return nil, err
	}

	return pu, nil
}

func (u *UserDomainSVC) SignInByCode(ctx context.Context, authType, studentID, unitId, verify string) (*user.User, error) {
	synpResp, err := u.Synp4bCli.Login(ctx, authType, studentID, unitId, verify)
	if err != nil {
		return nil, err
	}

	// 查询本地账号
	pu, err := u.findPsychUser(ctx, synpResp)
	if err != nil || pu == nil {
		return nil, err
	}

	return pu, nil
}

func (u *UserDomainSVC) CreateUser(ctx context.Context, unitId, email, phone, code, password string, psychUser *user.User) (*user.User, error) {
	// 创建basicUser
	bu, err := u.Synp4bCli.CreateBasicUser(ctx, unitId, code, phone, email, password, 0) // encryptType为0表示传入明文密码，中台使用bcrypt加密后存储
	if err != nil {
		return nil, err
	}

	// 创建psychUser
	oid, _ := bson.ObjectIDFromHex(bu.BasicUserID)
	psychUser.ID = oid

	// 校验重复创建
	if old, err := u.UsrMapper.FindOneById(ctx, oid); old != nil {
		return nil, errorx.New(errno.ErrCreateUser, errorx.KV("field", "用户已存在"))
	} else if err != nil {
		return nil, err
	}

	err = u.UsrMapper.Insert(ctx, psychUser)
	if err != nil {
		logs.Error("[user mapper] insert new user failed", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrCreateUser, errorx.KV("field", "存储用户失败"))
	}

	return psychUser, nil
}

func (u *UserDomainSVC) UpdatePassword(ctx context.Context, userId, newPassword string) error {
	// TODO
	return errorx.New(errno.ErrUnImplement)
}

// 查询psychUser
func (u *UserDomainSVC) findPsychUser(ctx context.Context, synpResp *synapse.LoginResult) (*user.User, error) {
	// 查询psych用户
	Oids, err := util.ObjectIDsFromHex(synpResp.BasicUserID, synpResp.UnitID)
	if err != nil {
		logs.Error("[user domain] get basic user objectID from hex failed")
		return nil, err
	}

	usr, err := u.UsrMapper.FindOneById(ctx, Oids[0])
	if errors.Is(err, mongo.ErrNoDocuments) {
		logs.Errorf("[user mapper] psych user [id=%s] not found", synpResp.BasicUserID)
	} else if err != nil {
		logs.Error("[user mapper] FindOneById failed")
		return nil, err
	}

	return usr, err
}

func (u *UserDomainSVC) SendVerifyCode(ctx context.Context, authType, authId, cause string) error {
	err := u.Synp4bCli.SendVerifyCode(ctx, authType, authId, cause)
	if err != nil {
		return err
	}

	return nil
}
