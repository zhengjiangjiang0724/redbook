package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v3"
)

var ctx = context.Background()

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	TLS      bool   `yaml:"tls"`
}

type JWTConfig struct {
	Secret        string `yaml:"secret"`
	AccessExpire  int64  `yaml:"access_expire"`
	RefreshExpire int64  `yaml:"refresh_expire"`
}

type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	Redis  RedisConfig  `yaml:"redis"`
	JWT    JWTConfig    `yaml:"jwt"`
}

var GlobalConfig *Config
var RedisClient *redis.Client

func InitConfig(path string) {
	data, err := os.ReadFile(path + "/config.yaml")
	if err != nil {
		log.Fatalf("Read config failed: %v", err)
	}
	if err := yaml.Unmarshal(data, &GlobalConfig); err != nil {
		log.Fatalf("Parse config failed: %v", err)
	}
	applyEnvOverrides()
}

func InitRedis() {
	opt := &redis.Options{
		Addr:     GlobalConfig.Redis.Addr,
		Password: GlobalConfig.Redis.Password,
		DB:       GlobalConfig.Redis.DB,
	}
	if GlobalConfig.Redis.TLS {
		opt.TLSConfig = &tls.Config{}
	}
	RedisClient = redis.NewClient(opt)
	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		panic(fmt.Sprintf("Redis connect failed: %v", err))
	}
	fmt.Println("Redis connected!")
}

func applyEnvOverrides() {
	if GlobalConfig == nil {
		return
	}
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		GlobalConfig.MySQL.DSN = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		GlobalConfig.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		GlobalConfig.Redis.Password = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		GlobalConfig.Server.Port = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		GlobalConfig.JWT.Secret = v
	}
	if v := os.Getenv("JWT_ACCESS_EXPIRE"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			GlobalConfig.JWT.AccessExpire = parsed
		}
	}
	if v := os.Getenv("JWT_REFRESH_EXPIRE"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			GlobalConfig.JWT.RefreshExpire = parsed
		}
	}
}
