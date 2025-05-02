package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	ConfGoPath   string `yaml:"conf_go_path"`
	DataPath     string `yaml:"data_path"`
	MetadataPath string `yaml:"metadata_path"`
}

func LoadConfig(filePath string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("LoadConfig falid: %v", err)
	}

	// 解析配置文件
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("LoadConfig falid: %v", err)
	}

	return &config, nil
}
