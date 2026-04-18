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
}

type RabbitMQ struct {
	URL string
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
