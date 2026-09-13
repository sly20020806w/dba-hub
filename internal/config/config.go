package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 平台配置
type Config struct {
	HTTPListen  string `yaml:"httpListen"`
	Token       string `yaml:"token"`
	DB          DB     `yaml:"db"`
	Notify      Notify `yaml:"notify"`
	Inspect     Inspect `yaml:"inspect"`
}

type DB struct {
	Driver string `yaml:"driver"` // sqlite | mysql（平台自身元数据库）
	DSN    string `yaml:"dsn"`
}

type Notify struct {
	FeishuWebhook string `yaml:"feishuWebhook"` // 巡检报告推送（可选）
}

type Inspect struct {
	IntervalS int `yaml:"intervalS"` // 自动巡检周期，0 表示关闭
}

func Default() *Config {
	return &Config{
		HTTPListen: ":8091",
		DB:         DB{Driver: "sqlite", DSN: "data/hub.db"},
		Inspect:    Inspect{IntervalS: 0},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil // 配置缺失时用默认值，降低上手门槛
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
