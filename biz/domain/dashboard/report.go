package dashboard

import (
	"context"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetReport 查看报表详情（班主任需目标学生在所带班级）
func (d *DashboardDomain) GetReport(ctx context.Context, scope *Scope, convOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	if scope.IsClassTeacher() {
		rs, err := d.resolveScope(ctx, scope)
		if err != nil {
			return nil, err
		}
		if err := d.ensureStudentInScope(ctx, rs, targetUser); err != nil {
			return nil, err
		}
	}
	return d.getReport(ctx, convOID, req)
}

// getReport 报表详情
func (d *DashboardDomain) getReport(ctx context.Context, convOID bson.ObjectID, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	rpt, err := d.ReportMapper.FindByConversationPreferSuccess(ctx, convOID)
	if err != nil {
		logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetReport)
	}

	resp := &core_api.DashboardGetReportResp{
		ReportId:       rpt.ID.Hex(),
		Title:          rpt.Title,
		Topics:         rpt.Topics,
		Digest:         rpt.Digest,
		Emotion:        int32(rpt.Emotion),
		Body:           rpt.Body,
		Suggestions:    rpt.Suggestions,
		NeedAlarm:      rpt.NeedAlarm,
		KeywordPercent: rpt.Keywords,
		ReportStatus:   int32(rpt.Status),
		Analysis:       analysisToPB(rpt.Analysis),
		SimpleReport:   simpleReportToPB(rpt.SimpleReport),
		Code:           0,
		Msg:            "success",
	}
	if rpt.Character != nil {
		resp.CharacterId = rpt.Character.ID.Hex()
		resp.CharacterName = rpt.Character.Name
		resp.CharacterVoice = rpt.Character.Voice
		resp.CharacterImage = rpt.Character.Image
	}

	if rpt.Status != enum.ReportStatusSuccess {
		resp.Code = errno.ErrReportNotReady
		resp.Msg = "报表处理中，请稍后"
	}

	return resp, nil
}
