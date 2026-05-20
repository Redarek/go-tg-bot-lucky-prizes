package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken       string
	AdminIDs            []int64
	ShopURL             string
	SubChannelID        int64
	SubChannelLink      string
	ContestChannel1ID   int64
	ContestChannel1Link string
	ContestChannel2ID   int64
	ContestChannel2Link string

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

	adminIDs := parseAdminIDs(os.Getenv("ADMIN_ID"))

	subChannelID, err := strconv.ParseInt(os.Getenv("SUB_CHANNEL_ID"), 10, 64)
	if err != nil {
		log.Fatal("SUB_CHANNEL_ID должен быть числом (-100…): ", err)
	}

	contestChannel1ID := parseInt64OrZero("CONTEST_CHANNEL_1_ID")
	contestChannel2ID := parseInt64OrZero("CONTEST_CHANNEL_2_ID")

	return &Config{
		TelegramToken:       os.Getenv("TELEGRAM_APITOKEN"),
		AdminIDs:            adminIDs,
		ShopURL:             os.Getenv("SHOP_URL"),
		SubChannelID:        subChannelID,
		SubChannelLink:      os.Getenv("SUB_CHANNEL_LINK"),
		ContestChannel1ID:   contestChannel1ID,
		ContestChannel1Link: os.Getenv("CONTEST_CHANNEL_1_LINK"),
		ContestChannel2ID:   contestChannel2ID,
		ContestChannel2Link: os.Getenv("CONTEST_CHANNEL_2_LINK"),

		PostgresHost:     os.Getenv("POSTGRES_HOST"),
		PostgresPort:     os.Getenv("POSTGRES_PORT"),
		PostgresUser:     os.Getenv("POSTGRES_USER"),
		PostgresPassword: os.Getenv("POSTGRES_PASSWORD"),
		PostgresDB:       os.Getenv("POSTGRES_DB"),
	}
}

func parseAdminIDs(raw string) []int64 {
	if strings.TrimSpace(raw) == "" {
		log.Fatal("ADMIN_ID не задан")
	}

	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			log.Fatal("Ошибка при чтении ADMIN_ID: ", err)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		ids = append(ids, id)
		seen[id] = struct{}{}
	}

	if len(ids) == 0 {
		log.Fatal("ADMIN_ID пуст")
	}

	return ids
}

func parseInt64OrZero(name string) int64 {
	raw := os.Getenv(name)
	if raw == "" {
		return 0
	}

	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		log.Fatalf("%s должен быть числом (-100…): %v", name, err)
	}
	return v
}
