package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Worker struct {
	Host string `yaml:"host"`
}

type Config struct {
	Workers []Worker `yaml:"workers"`
	Setup   string   `yaml:"setup"`
	Run     string   `yaml:"run"`
}

func Load(configPath string) Config {
	b, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read configuration file: %v", err)
	}
	var config Config
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		log.Fatalf("Failed to parse configuration file: %v", err)
	}
	return config
}
