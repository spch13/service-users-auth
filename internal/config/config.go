package config

import (
	"sync"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type (
	Config struct {
		App App
	}

	App struct {
		HOST   string `env:"SERVICE_HOST,required"`
		PORT   string `env:"SERVICE_PORT,required"`
		GWPORT string `env:"SERVICE_GW_PORT,required"`
	}
)

var (
	instance Config
	once     sync.Once
)

func GetConfig() (Config, error) {
	var err error
	once.Do(func() {
		instance = Config{}
		err = godotenv.Load()
		if err != nil {
			return
		}

		err = env.Parse(&instance)
		if err != nil {
			return
		}
	})

	return instance, err
}
