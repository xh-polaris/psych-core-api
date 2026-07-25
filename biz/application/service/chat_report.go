package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/lock"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var _ IChatReportService = (*ChatReportService)(nil)

type IChatReportService interface {
	ChatReportStream(ctx context.Context, req *ChatReportReq) (*ChatReportStream, error)
}

type ChatReportService struct {
	AuthDomain         auth.IAuthDomain
	ReportMapper       report.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	UserMapper         user.IMongoMapper
	ConfigMapper       config.IMongoMapper
	Cache              cache.Cmdable
}

var ChatReportServiceSet = wire.NewSet(
	wire.Struct(new(ChatReportService), "*"),
	wire.Bind(new(IChatReportService), new(*ChatReportService)),
)

const (
	systemPrompt = `你是帮助驻校心理老师深度考察学生心理状态的助手。请根据提供的学生聊天记录和心理评估报告内容，为心理老师的工作提供专业的分析和建议。
你需要：
1. 仔细阅读学生的对话记录和评估报告
2. 结合报告中分析的心理状态、情绪、风险等信息
3. 回答心理老师的问题，提供有针对性的建议和指导
4. 用专业但易于理解的语言进行交流
5. 如发现危急情况（自伤、自杀等），请明确提醒老师注意`

	sessionKeyPrefix = "teacher_chat_session:"
	sessionTTL       = 30 * time.Minute
	maxStudentMsgs   = 30
)

type ChatReportReq struct {
	ConversationId string `json:"conversationId"`
	SessionId      string `json:"sessionId,omitempty"`
	Message        string `json:"message"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type teacherChatSession struct {
	ConversationId string    `json:"conversationId"`
	UnitId         string    `json:"unitId"`
	StudentUserId  string    `json:"studentUserId"`
	ReportText     string    `json:"reportText"`
	StudentMsgs    string    `json:"studentMsgs"`
	History        []chatMsg `json:"history"`
}

type ChatReportDelta struct {
	Delta     string `json:"delta"`
	Finish    bool   `json:"finish"`
	SessionId string `json:"sessionId,omitempty"`
}

type ChatReportStream struct {
	reader *schema.StreamReader[*schema.Message]
	unlock func()
	sess   *teacherChatSession
	cache  cache.Cmdable
}

func (s *ChatReportStream) Recv() (*ChatReportDelta, bool, error) {
	msg, err := s.reader.Recv()
	if err == io.EOF {
		s.saveSession()
		s.unlock()
		return &ChatReportDelta{Finish: true}, true, nil
	}
	if err != nil {
		s.unlock()
		return nil, false, err
	}
	return &ChatReportDelta{Delta: msg.Content, Finish: false}, false, nil
}

func (s *ChatReportStream) saveSession() {
	key := sessionKeyPrefix + s.sess.ConversationId
	data, _ := json.Marshal(s.sess)
	_ = s.cache.Set(context.Background(), key, data, sessionTTL).Err()
}

func (s *ChatReportService) ChatReportStream(ctx context.Context, req *ChatReportReq) (*ChatReportStream, error) {
	if req.Message == "" {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "消息不能为空"))
	}

	m, err := s.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	convOID, err := bson.ObjectIDFromHex(req.ConversationId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "conversationId"))
	}

	conv, err := s.ConversationMapper.FindOneById(ctx, convOID)
	if err != nil || conv == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "对话"))
	}

	studentUserOID := conv.UserID
	studentUser, err := s.UserMapper.FindOneById(ctx, studentUserOID)
	if err != nil || studentUser == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "学生用户"))
	}
	unitOID := studentUser.UnitID

	if !m.HasSuperAdminAuth() {
		if m.Role < enum.UserRoleTeacher {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		if m.UnitId != unitOID.Hex() {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
	}

	lockKey := fmt.Sprintf("teacher_chat:%s:%s", unitOID.Hex(), studentUserOID.Hex())
	dLock := lock.Mgr.NewLock(lockKey)
	ok, lockErr := dLock.TryLock(ctx, 5*time.Minute, 5*time.Second, 3*time.Minute)
	if lockErr != nil || !ok {
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "获取分布式锁失败"))
	}
	unlock := func() { _ = dLock.TryUnlock(context.Background()) }

	var sess *teacherChatSession
	if req.SessionId != "" && req.SessionId == req.ConversationId {
		sess, err = s.loadSession(ctx, req.ConversationId)
		if err != nil {
			unlock()
			return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "加载会话失败"))
		}
	}

	if sess == nil {
		sess, err = s.createSession(ctx, convOID, unitOID, studentUserOID)
		if err != nil {
			unlock()
			return nil, err
		}
	}

	eMsgs := s.buildMessages(sess, req.Message)

	chatConf, chatApp, err := s.initLLM(ctx, unitOID)
	if err != nil {
		unlock()
		return nil, err
	}
	_ = chatConf

	stream, err := chatApp.Stream(ctx, eMsgs)
	if err != nil {
		unlock()
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "对话模型调用失败"))
	}

	sess.History = append(sess.History, chatMsg{Role: "user", Content: req.Message})

	return &ChatReportStream{
		reader: stream,
		unlock: unlock,
		sess:   sess,
		cache:  s.Cache,
	}, nil
}

func (s *ChatReportService) loadSession(ctx context.Context, conversationId string) (*teacherChatSession, error) {
	key := sessionKeyPrefix + conversationId
	data, err := s.Cache.Get(ctx, key).Result()
	if err != nil || data == "" {
		return nil, nil
	}
	var sess teacherChatSession
	if err := json.Unmarshal([]byte(data), &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *ChatReportService) createSession(ctx context.Context, convOID, unitOID, studentUserOID bson.ObjectID) (*teacherChatSession, error) {
	rpt, err := s.ReportMapper.FindByConversationPreferSuccess(ctx, convOID)
	if err != nil || rpt == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "学生评估报告"))
	}

	reportText := formatReport(rpt)
	studentMsgsText := s.formatStudentMessages(ctx, convOID.Hex())

	return &teacherChatSession{
		ConversationId: convOID.Hex(),
		UnitId:         unitOID.Hex(),
		StudentUserId:  studentUserOID.Hex(),
		ReportText:     reportText,
		StudentMsgs:    studentMsgsText,
		History:        nil,
	}, nil
}

func (s *ChatReportService) formatStudentMessages(ctx context.Context, convId string) string {
	msgs, err := his.Mgr.RetrieveMessage(ctx, convId, maxStudentMsgs)
	if err != nil || len(msgs) == 0 {
		return "（暂无对话记录）"
	}

	var sb strings.Builder
	sb.WriteString("## 学生对话记录\n")
	for _, msg := range msgs {
		roleName := "未知"
		switch msg.Role {
		case enum.MsgRoleUser:
			roleName = "学生"
		case enum.MsgRoleAssistant:
			roleName = "AI心理老师"
		}
		sb.WriteString(fmt.Sprintf("【%s】: %s\n", roleName, msg.Content))
	}
	return sb.String()
}

func (s *ChatReportService) buildMessages(sess *teacherChatSession, newMsg string) []*schema.Message {
	eMsgs := []*schema.Message{
		{Role: schema.System, Content: systemPrompt + "\n\n" + sess.ReportText + "\n\n" + sess.StudentMsgs},
		{Role: schema.Assistant, Content: "我已仔细阅读以上学生的评估报告和对话记录，请心理老师提问，我会基于这些内容提供专业分析和建议。"},
	}

	for _, h := range sess.History {
		eMsgs = append(eMsgs, &schema.Message{
			Role:    schema.RoleType(h.Role),
			Content: h.Content,
		})
	}

	eMsgs = append(eMsgs, &schema.Message{
		Role:    schema.User,
		Content: newMsg,
	})

	return eMsgs
}

func (s *ChatReportService) initLLM(ctx context.Context, unitOID bson.ObjectID) (*app.ChatSetting, app.ChatApp, error) {
	cfg, err := s.ConfigMapper.FindOneByUnitID(ctx, unitOID)
	if err != nil || cfg == nil || cfg.Chat == nil {
		return nil, nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "单位Chat配置"))
	}

	globalCfg := conf.GetConfig()
	chatConf, err := globalCfg.ChatConf(&core_api.ChatApp{
		Provider: cfg.Chat.Provider,
		AppId:    cfg.Chat.AppID,
	})
	if err != nil {
		return nil, nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "获取Chat配置失败"))
	}

	chatApp, err := app.NewChatApp(ctx, uuid.New().String(), chatConf)
	if err != nil {
		return nil, nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "创建对话模型失败"))
	}

	return chatConf, chatApp, nil
}

func formatReport(rpt *report.Report) string {
	var sb strings.Builder
	sb.WriteString("## 学生心理评估报告\n\n")

	if rpt.Title != "" {
		sb.WriteString(fmt.Sprintf("**报告标题**: %s\n\n", rpt.Title))
	}
	if rpt.Digest != "" {
		sb.WriteString(fmt.Sprintf("**对话摘要**: %s\n\n", rpt.Digest))
	}
	if len(rpt.Topics) > 0 {
		sb.WriteString(fmt.Sprintf("**主要话题**: %s\n\n", strings.Join(rpt.Topics, "、")))
	}
	if rpt.Body != "" {
		sb.WriteString(fmt.Sprintf("**对话正文**: %s\n\n", rpt.Body))
	}
	if len(rpt.Suggestions) > 0 {
		sb.WriteString(fmt.Sprintf("**已有建议**: %s\n\n", strings.Join(rpt.Suggestions, "；")))
	}

	if sr := rpt.SimpleReport; sr != nil {
		sb.WriteString("### 简易报告\n\n")
		if sr.MainProblem != "" {
			sb.WriteString(fmt.Sprintf("- **主要问题**: %s\n", sr.MainProblem))
		}
		if sr.Emotion.Type != "" || sr.Emotion.Intensity != "" {
			sb.WriteString(fmt.Sprintf("- **情绪状态**: %s（%s）\n", sr.Emotion.Type, sr.Emotion.Intensity))
		}
		if sr.Thoughts != "" {
			sb.WriteString(fmt.Sprintf("- **认知模式**: %s\n", sr.Thoughts))
		}
		if len(sr.Behaviors) > 0 {
			sb.WriteString(fmt.Sprintf("- **行为表现**: %s\n", strings.Join(sr.Behaviors, "、")))
		}
		if len(sr.Needs) > 0 {
			sb.WriteString(fmt.Sprintf("- **表达需求**: %s\n", strings.Join(sr.Needs, "、")))
		}
		if sr.Duration != "" {
			sb.WriteString(fmt.Sprintf("- **问题持续时间**: %s\n", sr.Duration))
		}
		if sr.FunctionImpact != "" {
			sb.WriteString(fmt.Sprintf("- **功能影响**: %s\n", sr.FunctionImpact))
		}
		if sr.Triggers != "" {
			sb.WriteString(fmt.Sprintf("- **诱发因素**: %s\n", sr.Triggers))
		}
		if sr.Coping != "" {
			sb.WriteString(fmt.Sprintf("- **应对方式**: %s\n", sr.Coping))
		}
		if sr.Support != "" {
			sb.WriteString(fmt.Sprintf("- **支持系统**: %s\n", sr.Support))
		}
		if sr.HelpSeeking != "" {
			sb.WriteString(fmt.Sprintf("- **求助意愿**: %s\n", sr.HelpSeeking))
		}
		if sr.RiskObservation.Level != "" {
			sb.WriteString(fmt.Sprintf("- **风险观察**: 等级=%s", sr.RiskObservation.Level))
			if sr.RiskObservation.Evidence != "" {
				sb.WriteString(fmt.Sprintf("，证据=%s", sr.RiskObservation.Evidence))
			}
			sb.WriteString("\n")
		}
		if sr.SeverityAssessment.Level != "" {
			sb.WriteString(fmt.Sprintf("- **严重程度**: %s（%s）\n", sr.SeverityAssessment.Level, sr.SeverityAssessment.Basis))
		}
		if sr.Summary.MainProblem != "" || sr.Summary.RiskLevel != "" {
			sb.WriteString(fmt.Sprintf("- **总结**: 主要问题=%s，情绪=%s，严重程度=%s，风险=%s，关注点=%s\n",
				sr.Summary.MainProblem, sr.Summary.EmotionState, sr.Summary.Severity, sr.Summary.RiskLevel, sr.Summary.Focus))
		}
		if sr.ProvidedSupport != "" {
			sb.WriteString(fmt.Sprintf("- **已提供支持**: %s\n", sr.ProvidedSupport))
		}
		if len(sr.Suggestions) > 0 {
			sb.WriteString(fmt.Sprintf("- **给教师的建议**: %s\n", strings.Join(sr.Suggestions, "；")))
		}
		sb.WriteString("\n")
	}

	if a := rpt.Analysis; a != nil {
		sb.WriteString("### 详细分析\n\n")
		if a.Problem.Primary.Category != "" {
			sb.WriteString(fmt.Sprintf("- **问题分类**: %s / %s\n", a.Problem.Primary.Category, a.Problem.Primary.Subcategory))
		}
		if len(a.Problem.Secondary) > 0 {
			var secs []string
			for _, s := range a.Problem.Secondary {
				secs = append(secs, fmt.Sprintf("%s/%s", s.Category, s.Subcategory))
			}
			sb.WriteString(fmt.Sprintf("- **次要问题**: %s\n", strings.Join(secs, "、")))
		}
		if len(a.Emotion.Types) > 0 || a.Emotion.Intensity != "" {
			sb.WriteString(fmt.Sprintf("- **情绪分析**: 类型=%s，强度=%s\n", strings.Join(a.Emotion.Types, "、"), a.Emotion.Intensity))
		}
		if len(a.Cognition) > 0 {
			sb.WriteString(fmt.Sprintf("- **认知模式**: %s\n", strings.Join(a.Cognition, "、")))
		}
		if len(a.Behavior) > 0 {
			sb.WriteString(fmt.Sprintf("- **行为表现**: %s\n", strings.Join(a.Behavior, "、")))
		}
		if a.Duration != "" {
			sb.WriteString(fmt.Sprintf("- **持续时间**: %s\n", a.Duration))
		}
		if len(a.Trigger) > 0 {
			sb.WriteString(fmt.Sprintf("- **诱发因素**: %s\n", strings.Join(a.Trigger, "、")))
		}
		if len(a.Coping) > 0 {
			sb.WriteString(fmt.Sprintf("- **应对策略**: %s\n", strings.Join(a.Coping, "、")))
		}
		if a.Support.Family || a.Support.Teacher || a.Support.Friend {
			var supports []string
			if a.Support.Family {
				supports = append(supports, "家庭")
			}
			if a.Support.Teacher {
				supports = append(supports, "教师")
			}
			if a.Support.Friend {
				supports = append(supports, "同伴")
			}
			sb.WriteString(fmt.Sprintf("- **支持系统**: %s\n", strings.Join(supports, "、")))
		}
		if a.HelpSeeking != "" {
			sb.WriteString(fmt.Sprintf("- **求助意愿**: %s\n", a.HelpSeeking))
		}
		if a.Function.Learning != "" || a.Function.Sleep != "" || a.Function.Diet != "" || a.Function.Interpersonal != "" || a.Function.DailyLife != "" {
			sb.WriteString(fmt.Sprintf("- **功能影响**: 学习=%s，睡眠=%s，饮食=%s，人际=%s，日常生活=%s\n",
				a.Function.Learning, a.Function.Sleep, a.Function.Diet, a.Function.Interpersonal, a.Function.DailyLife))
		}
		if a.Distress.Level != "" {
			sb.WriteString(fmt.Sprintf("- **痛苦程度**: %s（%s）\n", a.Distress.Level, strings.Join(a.Distress.Reason, "、")))
		}
		if a.Risk.Level != "" {
			sb.WriteString(fmt.Sprintf("- **风险评估**: 等级=%s，总分=%d\n", a.Risk.Level, a.Risk.Score.Total))
			if len(a.Risk.Evidence) > 0 {
				sb.WriteString(fmt.Sprintf("  证据: %s\n", strings.Join(a.Risk.Evidence, "；")))
			}
			if a.Risk.Action != "" {
				sb.WriteString(fmt.Sprintf("  建议行动: %s\n", a.Risk.Action))
			}
		}
		if a.Confidence.Overall != "" {
			sb.WriteString(fmt.Sprintf("- **评估置信度**: 总体=%s，风险=%s\n", a.Confidence.Overall, a.Confidence.Risk))
		}
	}

	return sb.String()
}
