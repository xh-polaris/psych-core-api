package conf

import (
	"fmt"
	"os"
	"strconv"

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
	Prefix   string   // 密钥格式中的前缀，可用于日志关联
	Digest   string   // 密钥随机部分的 HMAC-SHA256 十六进制摘要
	Status   string   // 启用或吊销
	Scopes   []string // 例如 psych:chat
	Upstream string   // 上游 Key 名，引用 OpenApi.Upstreams；必填
}

// UpstreamKey 上游模型服务凭据，每个条目对应一把 DeepSeek Key。
// 该条目以名字而非明文被 OpenApiKey.Upstream 引用，因此报告链路
// 可以把名字透传给 psych-post，由对端自行解析，避免明文跨服务。
type UpstreamKey struct {
	URL       string
	Model     string
	AccessKey string
}

type OpenApi struct {
	Pepper               string
	Keys                 []OpenApiKey
	Upstreams            map[string]*UpstreamKey `json:",optional"` // 命名上游注册表
	MaxMessages          int                     // 消息最大条数
	MaxMessageChars      int                     // 单条消息最大字符数
	MaxTotalChars        int                     // 全部消息总字符数
	ReportInternalURL    string                  // psych-post 集群内私有报告接口地址
	ReportInternalToken  string                  // 调用私有接口的 Bearer Token
	ReportTimeoutSeconds int                     // 私有调用超时秒数
}

type PostProcess struct {
	Enabled bool
}

type Config struct {
	service.ServiceConf
	ListenOn    string
	State       string
	PostProcess *PostProcess `json:",optional"`
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

func (c *Config) PostProcessEnabled() bool {
	return c.State != "test" || (c.PostProcess != nil && c.PostProcess.Enabled)
}

// validateOpenAPIUpstreams 校验开放接口的密钥与上游映射完整性。
// 上游名写错只会让每个请求退化成 502，排查成本高，因此在启动期拦截。
// 一把平台 Key 只能绑定一把独立上游 Key；不允许多个平台 Key 共享同一凭据。
// 抽成不依赖 I/O 的纯函数，便于用字面量构造的 Config 直接单测。
func (c *Config) validateOpenAPIUpstreams() error {
	if c.OpenApi == nil || len(c.OpenApi.Keys) == 0 {
		return nil
	}
	if c.OpenApi.Pepper == "" {
		return fmt.Errorf("openapi: Pepper is required when Keys are configured")
	}
	boundUpstreams := make(map[string]string, len(c.OpenApi.Keys))
	for _, key := range c.OpenApi.Keys {
		if key.Upstream == "" {
			return fmt.Errorf("openapi: key %s has no Upstream", key.Prefix)
		}
		upstream, ok := c.OpenApi.Upstreams[key.Upstream]
		if !ok || upstream == nil {
			return fmt.Errorf("openapi: key %s references undefined upstream %q", key.Prefix, key.Upstream)
		}
		if upstream.URL == "" || upstream.Model == "" || upstream.AccessKey == "" {
			return fmt.Errorf("openapi: upstream %q is incomplete", key.Upstream)
		}
		if boundBy, exists := boundUpstreams[key.Upstream]; exists {
			return fmt.Errorf("openapi: keys %s and %s share upstream %q", boundBy, key.Prefix, key.Upstream)
		}
		boundUpstreams[key.Upstream] = key.Prefix
	}
	return nil
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
	// 本地联调可通过环境变量覆盖集群内地址，无需复制含密钥的配置文件。
	if value := os.Getenv("PSYCH_LISTEN_ON"); value != "" {
		c.ListenOn = value
	}
	if value := os.Getenv("PSYCH_MONGO_URL"); value != "" && c.Mongo != nil {
		c.Mongo.URL = value
	}
	if value := os.Getenv("PSYCH_REDIS_ADDR"); value != "" {
		if c.Cache != nil {
			c.Cache.Addr = value
		}
		for i := range c.CacheConf {
			cacheDB := 0
			if c.Cache != nil {
				cacheDB = c.Cache.DB
			}
			c.CacheConf[i].Host = value + "/" + strconv.Itoa(cacheDB)
		}
	}
	if value := os.Getenv("PSYCH_RABBITMQ_URL"); value != "" && c.RabbitMQ != nil {
		c.RabbitMQ.URL = value
	}
	if os.Getenv("PSYCH_DISABLE_TELEMETRY") == "1" {
		c.Telemetry.Disabled = true
	}
	// 在 SetUp 之前校验：SetUp 会拉起 logx/prometheus/trace 等 agent，
	// 配置错误时应尽早返回，避免留下半初始化的运行时。
	if err = c.validateOpenAPIUpstreams(); err != nil {
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
