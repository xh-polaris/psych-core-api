package conf

type SMS struct {
	MaxInPeriod   int               // 周期内最多发送次数
	Period        int               // 周期, 单位: 秒
	Provider      string            // 供应商, 如 "tencent"
	Account       string            // 腾讯云 SecretId
	Token         string            // 腾讯云 SecretKey
	Extra         map[string]string // 不同渠道的额外信息, 如 AppId/Sign 等
	CauseTemplate map[string]string `json:",default={}"` // cause → 模板ID 映射, 如 alert → "12345"
}
