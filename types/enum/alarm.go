package enum

// AlarmStatus
const (
	AlarmStatusProcessed = 1
	AlarmStatusPending   = 2
)

// AlarmEmotion 应按照严重程度升序
const (
	UnknownEmotion = iota
	AlarmEmotionDanger
	AlarmEmotionDepress
	AlarmEmotionAnxiety
	AlarmEmotionNegative
	AlarmEmotionNormal
)
