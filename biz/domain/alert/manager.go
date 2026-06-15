package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/sms_alert"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/sms"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var Mgr *AlertManager

type AlertManager struct {
	cache          cache.Cmdable
	userMapper     user.IMongoMapper
	configMapper   config.IMongoMapper
	unitMapper     unit.IMongoMapper
	smsAlertMapper sms_alert.IMongoMapper
}

func New(c cache.Cmdable, userMapper user.IMongoMapper, configMapper config.IMongoMapper, unitMapper unit.IMongoMapper, smsAlertMapper sms_alert.IMongoMapper) {
	Mgr = &AlertManager{
		cache:          c,
		userMapper:     userMapper,
		configMapper:   configMapper,
		unitMapper:     unitMapper,
		smsAlertMapper: smsAlertMapper,
	}
}

// CheckAndSetLimit 检测今日是否已发过告警短信，若未发过则设置标记。
// 返回 true 表示未超限可发送，false 表示今日已发。
func (m *AlertManager) CheckAndSetLimit(ctx context.Context, userId string) (bool, error) {
	date := util.FormatDateUTC8(time.Now())
	key := fmt.Sprintf("%s:%s:%s", cst.AlertSmsLimitPrefix, date, userId)

	ok, err := m.cache.SetNX(ctx, key, "1", 24*time.Hour).Result()
	if err == nil {
		return ok, nil
	}

	logs.Warnf("[alert] Redis SetNX failed, fallback to DB: %v", err)

	userOid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return false, err
	}
	start, end, err := util.DayToUTCRange(date)
	if err != nil {
		return false, err
	}
	cnt, err := m.smsAlertMapper.CountTodayByUserId(ctx, userOid, start, end)
	if err != nil {
		return false, err
	}
	return cnt == 0, nil
}

// ResolveRecipients 解析告警短信接收人手机号列表。
func (m *AlertManager) ResolveRecipients(ctx context.Context, unitId bson.ObjectID, student *user.User) []string {
	var phones []string

	cfg, err := m.configMapper.FindOneByUnitID(ctx, unitId)
	if err == nil && cfg != nil && len(cfg.AlertPhone) > 0 {
		phones = append(phones, cfg.AlertPhone[0])
	} else if err != nil {
		logs.Warnf("[alert] resolve config AlertPhone err: %v", err)
	}

	ct, err := m.userMapper.FindClassTeacherOfStudent(ctx, unitId, student.EnrollYear, student.Class)
	if err == nil && ct != nil && ct.CodeType == enum.UserCodeTypePhone {
		phones = append(phones, ct.Code)
	} else if err != nil {
		logs.Warnf("[alert] resolve class teacher err: %v", err)
	}

	seen := make(map[string]struct{}, len(phones))
	deduped := make([]string, 0, len(phones))
	for _, p := range phones {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			deduped = append(deduped, p)
		}
	}
	return deduped
}

// Send 发送告警短信并持久化记录。
func (m *AlertManager) Send(ctx context.Context, unitId, userId, convId bson.ObjectID) error {
	userIdHex := userId.Hex()

	student, err := m.userMapper.FindOneById(ctx, userId)
	if err != nil {
		logs.Errorf("[alert] find student err: %v", err)
		return errorx.WrapByCode(err, errno.AlertResolve)
	}

	unitInfo, err := m.unitMapper.FindOneById(ctx, unitId)
	if err != nil {
		logs.Errorf("[alert] find unit err: %v", err)
		return errorx.WrapByCode(err, errno.AlertResolve)
	}

	grade := util.CalculateGrade(unitInfo.StartGrade, student.EnrollYear)
	gradeStr := fmt.Sprintf("%d", grade)
	classStr := fmt.Sprintf("%d", student.Class)

	phones := m.ResolveRecipients(ctx, unitId, student)
	if len(phones) == 0 {
		logs.Warnf("[alert] no recipients resolved for user %s", userIdHex)
		return nil
	}

	ok, err := m.CheckAndSetLimit(ctx, userIdHex)
	if err != nil {
		logs.Errorf("[alert] check limit err: %v", err)
		return errorx.WrapByCode(err, errno.AlertSms)
	}
	if !ok {
		logs.Infof("[alert] user %s already reached daily sms limit", userIdHex)
		return nil
	}

	params := []string{student.Name, gradeStr, classStr}

	var sendErr error
	var successPhones []string
	for _, phone := range phones {
		if e := sms.Mgr.Send(ctx, "alert", phone, params); e != nil {
			logs.Errorf("[alert] send sms to %s err: %v", phone, e)
			sendErr = e
		} else {
			successPhones = append(successPhones, phone)
		}
	}

	if len(successPhones) > 0 {
		record := &sms_alert.SmsAlert{
			ID:         bson.NewObjectID(),
			UnitID:     unitId,
			UserID:     userId,
			RecvPhone:  successPhones,
			ConvID:     convId,
			CreateTime: time.Now(),
		}
		if err := m.smsAlertMapper.Insert(ctx, record); err != nil {
			logs.Errorf("[alert] insert sms_alert record err: %v", err)
		}
	}

	if sendErr != nil && len(successPhones) == 0 {
		return errorx.WrapByCode(sendErr, errno.AlertSmsSend)
	}

	return nil
}
