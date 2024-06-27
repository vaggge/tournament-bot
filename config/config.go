package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
)

type Config struct {
	MongoURI    string
	BotToken    string
	WebhookURL  string
	Port        string
	IsLocalMode bool
}

func LoadConfig() *Config {
	// Загрузка .env файла
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file. Using environment variables.")
	}

	isLocalMode, _ := strconv.ParseBool(getEnvOrPanic("LOCAL_MODE"))

	config := &Config{
		MongoURI:    getEnvOrPanic("MONGO_URI"),
		BotToken:    getEnvOrPanic("BOT_TOKEN"),
		Port:        getEnvOrPanic("PORT"),
		WebhookURL:  getEnvOrPanic("WEBHOOK_URL"),
		IsLocalMode: isLocalMode,
	}

	if !isLocalMode {
		config.WebhookURL = getEnvOrPanic("WEBHOOK_URL")
	}

	return config
}

func getEnvOrPanic(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("Environment variable %s is not set", key)
	}
	return value
}
