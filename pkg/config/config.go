package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken  string
	AdminID        int64
	ShopURL        string
	SubChannelID   int64
	SubChannelLink string

	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Файл .env не найден, используются переменные окружения системы.")
	}

	adminID, err := strconv.ParseInt(os.Getenv("ADMIN_ID"), 10, 64)
	if err != nil {
		log.Fatal("Ошибка при чтении ADMIN_ID: ", err)
	}

	subChannelID, err := strconv.ParseInt(os.Getenv("SUB_CHANNEL_ID"), 10, 64)
	if err != nil {
		log.Fatal("SUB_CHANNEL_ID должен быть числом (-100…): ", err)
	}

	return &Config{
		TelegramToken:  os.Getenv("TELEGRAM_APITOKEN"),
		AdminID:        adminID,
		ShopURL:        os.Getenv("SHOP_URL"),
		SubChannelID:   subChannelID,
		SubChannelLink: os.Getenv("SUB_CHANNEL_LINK"),

		PostgresHost:     os.Getenv("POSTGRES_HOST"),
		PostgresPort:     os.Getenv("POSTGRES_PORT"),
		PostgresUser:     os.Getenv("POSTGRES_USER"),
		PostgresPassword: os.Getenv("POSTGRES_PASSWORD"),
		PostgresDB:       os.Getenv("POSTGRES_DB"),
	}
}
