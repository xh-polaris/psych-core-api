package conf

import (
	"os"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/cache"
)

var config *Config

type Auth struct {
	SecretKey    string
	PublicKey    string
	AccessExpire int64
}

type Cache struct {
	Addr     string
	Password string
	DB       int `json:"DB,omitempty,default=0"`
}

type RabbitMQ struct {
	URL        string
	Exchange   string
	RoutingKey string
}

type Mongo struct {
	URL string
	DB  string
}

type Synapse struct {
	BaseURL   string
	CreateKey string
	State     string
}

type COS struct {
	BucketURL string
	CDN       string `json:",optional"`
	SecretID  string
	SecretKey string
}

type OpenApiKey struct {
	Prefix string   // 密钥格式中的前缀，可用于日志关联
	Digest string   // 密钥随机部分的 HMAC-SHA256 十六进制摘要
	Status string   // 启用或吊销
	Scopes []string // 例如 psych:chat
}

type OpenApi struct {
	Pepper               string
	Keys                 []OpenApiKey
	MaxMessages          int    // 消息最大条数
	MaxMessageChars      int    // 单条消息最大字符数
	MaxTotalChars        int    // 全部消息总字符数
	ReportInternalURL    string // psych-post 集群内私有报告接口地址
	ReportInternalToken  string // 调用私有接口的 Bearer Token
	ReportTimeoutSeconds int    // 私有调用超时秒数
}

type Config struct {
	service.ServiceConf
	ListenOn    string
	State       string
	Auth        Auth
	Cache       *Cache
	CacheConf   cache.CacheConf
	RabbitMQ    *RabbitMQ
	Mongo       *Mongo
	ModelConfig *ModelConfig
	Synapse     *Synapse
	SMS         *SMS
	COS         *COS
	OpenApi     *OpenApi `json:",optional"`
}

func NewConfig() (*Config, error) {
	c := new(Config)
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "etc/config.yaml"
	}
	err := conf.Load(path, c)
	if err != nil {
		return nil, err
	}
	err = c.SetUp()
	if err != nil {
		return nil, err
	}
	config = c
	return c, nil
}

func GetConfig() *Config {
	return config
}
