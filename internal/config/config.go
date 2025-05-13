package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server          ServerConfig     `mapstructure:"server"`
	GrpcServer      GrpcServerConfig `mapstructure:"grpc"`
	Database        DatabaseConfig   `mapstructure:"database"`
	Redis           RedisConfig      `mapstructure:"redis"`
	Ethereum        EthereumConfig   `mapstructure:"ethereum"`
	MQTT            MQTTConfig       `mapstructure:"mqtt"`
	Asynq           AsynqConfig      `mapstructure:"asynq"`
	Node            NodeConfig       `mapstructure:"node"`
	SecretKey       string
	SessionContract string `mapstructure:"session_receipt_address"`
	NodeContract    string `mapstructure:"node_storage_address"`
	ChainId         int    `mapstructure:"chain_id"`
	LogLevel        string `mapstructure:"log_level"`
	JwtSecretPath   string `mapstructure:"jwt_secret_path"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type GrpcServerConfig struct {
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

type MQTTConfig struct {
	BrokerURL      string        `mapstructure:"broker_url"`
	ClientID       string        `mapstructure:"client_id"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	QoS            byte          `mapstructure:"qos"`
	CleanSession   bool          `mapstructure:"clean_session"`
	PingInterval   time.Duration `mapstructure:"ping_interval"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

type AsynqConfig struct {
	RedisAddr   string `mapstructure:"redis_addr"`
	Concurrency int    `mapstructure:"concurrency"`
}

type NodeConfig struct {
	PrivateKeyPath string   `mapstructure:"private_key_path"`
	WhitelistPaths []string `mapstructure:"whitelist_paths"`
}

var cfg *Config

func Load() (*Config, error) {
	godotenv.Load()
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	viper.SetDefault("JSONRPC_HOST", "localhost")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	cfg.SecretKey = viper.GetString("SECRET_KEY")

	return cfg, nil
}

func Get() *Config {
	return cfg
}
