package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App App `mapstructure:"app"`
	DB  DB  `mapstructure:"db"`
}

type App struct {
	Mode    string `mapstructure:"mode"`
	Port    string `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
}

type DB struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Name     string `mapstructure:"name"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"ssl_mode"`
	TimeZone string `mapstructure:"time_zone"`
}

func Load() (*Config, error) {
	viper.SetConfigFile("config.yml")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config read error: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal error: %w", err)
	}

	return &cfg, nil
}

func (d DB) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		d.Host, d.User, d.Password, d.Name, d.Port, d.SSLMode, d.TimeZone,
	)
}
