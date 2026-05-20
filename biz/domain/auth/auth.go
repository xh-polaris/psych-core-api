package auth

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var _ IAuthDomain = (*AuthDomain)(nil)

// IAuthDomain 鉴权域
// 返回的错误直接透传
type IAuthDomain interface {
	// ExtraUserMeta 从 ctx 提取 JWT 中的用户信息
	ExtraUserMeta(ctx context.Context) (*Meta, error)

	// IdentifyRole 根据请求上下文确定操作者角色，返回 enum.UserRole 值
	// - unitId 为空：要求超管（JWT + DB 双重校验）
	// - unitId 非空：返回 UserRoleUnitAdmin 或 UserRoleClassTeacher
	IdentifyRole(ctx context.Context, unitId string) (*Meta, int, error)

	// VerifySuperAdmin 校验超管权限（JWT role + DB role 双重校验）
	VerifySuperAdmin(ctx context.Context, meta *Meta) error

	// VerifyUnitAdmin 校验单位管理员权限（含超管）
	VerifyUnitAdmin(meta *Meta, unitId string) error

	// VerifyClassTeacher 校验班主任权限
	VerifyClassTeacher(meta *Meta) error
}

type AuthDomain struct {
	UserMapper user.IMongoMapper
	UnitMapper unit.IMongoMapper
}

var AuthDomainSet = wire.NewSet(
	wire.Struct(new(AuthDomain), "*"),
	wire.Bind(new(IAuthDomain), new(*AuthDomain)),
)

// ExtraUserMeta 从ctx中提取出用户信息
func (a *AuthDomain) ExtraUserMeta(ctx context.Context) (*Meta, error) {
	var meta Meta
	c, err := httpx.ExtractContext(ctx)
	if err != nil {
		return nil, errorx.New(errno.ErrUnAuth)
	}
	claims, err := util.ParseJwt(string(c.GetHeader("Authorization")))
	if err != nil {
		return nil, err
	}
	meta.UserId = claims[cst.JsonUserID].(string)
	meta.UnitId = claims[cst.JsonUnitID].(string)
	meta.Code = claims[cst.JsonCode].(string)
	meta.Role = int(claims[cst.JsonRole].(float64))
	return &meta, nil
}

// IdentifyRole 根据请求上下文确定操作者角色 返回UserMeta和Role
// 若角色与操作要求不符则返回ErrInsufficientAuth
func (a *AuthDomain) IdentifyRole(ctx context.Context, unitId string) (*Meta, int, error) {
	usrMeta, err := a.ExtraUserMeta(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 无unitId：超管
	if unitId == "" {
		if err = a.VerifySuperAdmin(ctx, usrMeta); err != nil {
			return nil, 0, err
		}
		return usrMeta, enum.UserRoleSuperAdmin, nil
	}
	// 有unitId：单位管理员或班主任
	// 校验unit存在
	IDs, err := util.ObjectIDsFromHex(usrMeta.UserId, unitId)
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, errno.ErrNotFound)
	}
	_, err = a.UnitMapper.FindOneById(ctx, IDs[1])
	if err != nil {
		logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, 0, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "鉴权时UnitID"))
	}

	// 判断用户权限
	if usrMeta.IsUnitAdmin(unitId) {
		return usrMeta, enum.UserRoleUnitAdmin, nil
	}
	if usrMeta.IsClassTeacher(unitId) {
		return usrMeta, enum.UserRoleClassTeacher, nil
	}

	return nil, 0, errorx.New(errno.ErrInsufficientAuth)
}

// VerifySuperAdmin 校验超管权限：JWT role >= SuperAdmin 且 DB role == SuperAdmin
func (a *AuthDomain) VerifySuperAdmin(ctx context.Context, meta *Meta) error {
	if meta.Role < enum.UserRoleSuperAdmin {
		return errorx.New(errno.ErrInsufficientAuth)
	}
	oid, _ := bson.ObjectIDFromHex(meta.UserId)
	dbUser, err := a.UserMapper.FindOneById(ctx, oid)
	if err != nil || dbUser.Role != enum.UserRoleSuperAdmin {
		return errorx.New(errno.ErrInsufficientAuth)
	}
	return nil
}

// VerifyUnitAdmin 校验单位管理员权限（含超管）
func (a *AuthDomain) VerifyUnitAdmin(meta *Meta, unitId string) error {
	if meta.Role >= enum.UserRoleSuperAdmin {
		return nil
	}
	if meta.Role == enum.UserRoleUnitAdmin && meta.UnitId == unitId {
		return nil
	}
	return errorx.New(errno.ErrInsufficientAuth)
}

// VerifyClassTeacher 校验班主任权限
func (a *AuthDomain) VerifyClassTeacher(meta *Meta) error {
	if meta.Role >= enum.UserRoleClassTeacher {
		return nil
	}
	return errorx.New(errno.ErrInsufficientAuth)
}
