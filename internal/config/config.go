package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Ethereum  EthereumConfig  `mapstructure:"ethereum"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	Asynq     AsynqConfig     `mapstructure:"asynq"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type DatabaseConfig struct {
	URL string `mapstructure:"url"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type ChainConfig struct {
	RPCURL    string `mapstructure:"rpc_url"`
	BlockTime int    `mapstructure:"block_time"`
}

type EthereumConfig struct {
	Chains map[int]ChainConfig `mapstructure:"chains"`
}

type WebSocketConfig struct {
	PingInterval   int `mapstructure:"ping_interval"`
	PongWait       int `mapstructure:"pong_wait"`
	WriteWait      int `mapstructure:"write_wait"`
	MaxMessageSize int `mapstructure:"max_message_size"`
}

type AsynqConfig struct {
	RedisAddr   string `mapstructure:"redis_addr"`
	Concurrency int    `mapstructure:"concurrency"`
}

var cfg *Config

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return cfg, nil
}

func Get() *Config {
	return cfg
}
