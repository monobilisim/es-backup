package config

import (
	"flag"
	"github.com/spf13/viper"
	"log"
	"os"
)

type Params struct {
	Elasticsearch struct{
		Url string
		Username string
		Password string
		ApiKey string
		Tasks []struct{
			Repository string
			Indexes []string
			SnapshotName string
			TakenBy string
			TakenBecause string
			TimeoutByMinutes int
			Retention struct{
				ExpireAfter int
				MinCount int
				MaxCount int
			}
		}
	}
	Notify struct{
		Email struct{
			Enabled bool
			Info EmailConfig
			Error EmailConfig
		}
		Mattermost struct{
			Enabled bool
			Info MattermostConfig
			Error MattermostConfig
		}
	}
	Hostname string
}

type MattermostConfig struct {
	Url string
	ChannelId string
	ApiToken string
}

type EmailConfig struct {
	SmtpHost string
	SmtpPort int
	From string
	Password string
	To string
}

func NewParams() (p *Params) {
	filePath := flag.String("config", "/etc/es-backup.yml", "Path of the configuration file in YAML format")
	flag.Parse()

	if _, err := os.Stat(*filePath); os.IsNotExist(err) {
		log.Fatalf("Configuration file: %s does not exist, %v\n", *filePath, err)
	}

	viper.SetConfigFile(*filePath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	err := viper.Unmarshal(&p)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v\n", err)
	}

	p.Hostname, _ = os.Hostname()

	return
}
