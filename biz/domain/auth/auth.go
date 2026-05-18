package auth

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var _ IAuthDomain = (*AuthDomain)(nil)

// IAuthDomain 鉴权领域接口
type IAuthDomain interface {
	// ExtraUserMeta 从 ctx 提取 JWT 中的用户信息
	ExtraUserMeta(ctx context.Context) (*Meta, error)
	// VerifySuperAdmin 校验超管权限（JWT role + DB role 双重校验）
	VerifySuperAdmin(ctx context.Context, meta *Meta) error
	// VerifyUnitAdmin 校验单位管理员权限（含超管）
	VerifyUnitAdmin(meta *Meta, unitId string) error
	// VerifyClassTeacher 校验班主任权限
	VerifyClassTeacher(meta *Meta) error
}

// Meta 是jwt的claim负载，包含用户基础信息和权限等级
type Meta struct {
	UserId string `json:"userId"`
	UnitId string `json:"unitId;omitempty"`
	Code   string `json:"code;omitempty"`
	Role   int    `json:"role"` // 权限等级 (学生用户、老师、班主任、单位管理、超管)
}

func (m *Meta) HasUnitAdminAuth(unitId string) bool {
	return m.Role >= enum.UserRoleSuperAdmin || (m.Role == enum.UserRoleUnitAdmin && m.UnitId == unitId)
}

func (m *Meta) HasSuperAdminAuth() bool {
	return m.Role >= enum.UserRoleSuperAdmin
}

func (m *Meta) HasClassTeacherAuth() bool {
	return m.Role >= enum.UserRoleClassTeacher
}

func (m *Meta) HasTeacherAuth() bool {
	return m.Role >= enum.UserRoleTeacher
}

type AuthDomain struct {
	UserMapper user.IMongoMapper
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
