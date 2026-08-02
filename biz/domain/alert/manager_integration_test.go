package alert

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache/redis"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/sms_alert"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/sms"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const alertPhone = "17802160858"

// setupAlertManager 加载配置并初始化 AlertManager 所需依赖。
// 返回 cleanup 函数用于恢复全局变量。
func setupAlertManager(t *testing.T) (cache.Cmdable, user.IMongoMapper, unit.IMongoMapper, config.IMongoMapper, sms_alert.IMongoMapper, func()) {
	t.Helper()

	if os.Getenv("CONFIG_PATH") == "" {
		os.Setenv("CONFIG_PATH", "../../../etc/config.yaml")
	}
	c, err := conf.NewConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// 初始化带追踪的 Mongo Client
	if _, err := httpx.NewTracedClient(c.Mongo.URL); err != nil {
		t.Fatalf("init mongo client: %v", err)
	}

	userMapper := user.NewUserMongoMapper(c)
	unitMapper := unit.NewUnitMongoMapper(c)
	configMapper := config.NewConfigMongoMapper(c)
	smsAlertMapper := sms_alert.NewSmsAlertMongoMapper(c)

	cacheCli := redis.New()

	oldSmsMgr := sms.Mgr
	oldAlertMgr := Mgr

	sms.New(c)
	New(cacheCli, userMapper, configMapper, unitMapper, smsAlertMapper)

	cleanup := func() {
		sms.Mgr = oldSmsMgr
		Mgr = oldAlertMgr
	}
	return cacheCli, userMapper, unitMapper, configMapper, smsAlertMapper, cleanup
}

func TestSendSmsAlert_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, userMapper, unitMapper, configMapper, smsAlertMapper, cleanup := setupAlertManager(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// ── 创建测试数据 ──
	unitId := bson.NewObjectID()
	studentId := bson.NewObjectID()
	convId := bson.NewObjectID()

	// 单位
	if err := unitMapper.Insert(ctx, &unit.Unit{
		ID:         unitId,
		Name:       "测试学校",
		StartGrade: 1,
		Status:     enum.UnitStatusActive,
		CreateTime: now,
		UpdateTime: now,
	}); err != nil {
		t.Fatalf("insert unit: %v", err)
	}
	defer func() {
		_ = unitMapper.UpdateFields(ctx, unitId, bson.M{"$set": bson.M{"status": enum.UnitStatusDeleted}})
	}()

	// 小明，2025年入学，startGrade=1，当前(2026年6月) 年级 = 1 - 2025 + 2026 - 1 = 1
	student := &user.User{
		ID:         studentId,
		UnitID:     unitId,
		Name:       "小明",
		Code:       fmt.Sprintf("student_%s", studentId.Hex()),
		CodeType:   enum.UserCodeTypeStudentID,
		EnrollYear: 2025,
		Class:      1,
		Role:       enum.UserRoleStudent,
		Status:     enum.UserStatusActive,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := userMapper.Insert(ctx, student); err != nil {
		t.Fatalf("insert student: %v", err)
	}
	defer func() {
		_ = userMapper.UpdateFields(ctx, studentId, bson.M{"$set": bson.M{"status": enum.UserStatusDeleted}})
	}()

	// 班主任（手机号绑定，用于接收告警）
	teacherId := bson.NewObjectID()
	teacher := &user.User{
		ID:       teacherId,
		UnitID:   unitId,
		Name:     "王老师",
		Code:     alertPhone,
		CodeType: enum.UserCodeTypePhone,
		BindClasses: []user.ClassInfo{
			{EnrollYear: 2025, Class: 1},
		},
		Role:       enum.UserRoleClassTeacher,
		Status:     enum.UserStatusActive,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := userMapper.Insert(ctx, teacher); err != nil {
		t.Fatalf("insert teacher: %v", err)
	}
	defer func() {
		_ = userMapper.UpdateFields(ctx, teacherId, bson.M{"$set": bson.M{"status": enum.UserStatusDeleted}})
	}()

	// 单位告警配置（同时配置告警手机号）
	if err := configMapper.Insert(ctx, &config.Config{
		ID:         bson.NewObjectID(),
		UnitID:     unitId,
		AlertPhone: []string{alertPhone},
		Status:     enum.UnitStatusActive,
		CreateTime: now,
		UpdateTime: now,
	}); err != nil {
		t.Fatalf("insert config: %v", err)
	}

	// ── 发送告警短信 ──
	err := Mgr.Send(ctx, unitId, studentId, convId)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	t.Logf("告警短信已发送！接收人: %s, 学生: 小明 (1年级1班)", alertPhone)

	// 可选验证：检查 sms_alert 记录
	records, _ := smsAlertMapper.FindAllByFields(ctx, bson.M{"user_id": studentId})
	t.Logf("sms_alert 记录数: %d", len(records))
	for _, r := range records {
		t.Logf("  - unit=%s user=%s conv=%s recipients=%v time=%s",
			r.UnitID.Hex(), r.UserID.Hex(), r.ConvID.Hex(), r.Recipients, r.CreateTime.Format(time.RFC3339))
	}
}
