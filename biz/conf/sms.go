package conf

// SMS 短信服务配置
type SMS struct {
	MaxInPeriod int               // 周期内最多发送次数
	Period      int               // 周期, 单位: 秒
	Provider    string            // 供应商, 如 "tencent"
	Account     string            // 腾讯云 SecretId
	Token       string            // 腾讯云 SecretKey
	Extra       map[string]string // 额外信息, 如 Sign/TemplateId 等
}
