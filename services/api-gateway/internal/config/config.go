package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppEnv string
	Port   int

	Services ServicesConfig
}

type ServicesConfig struct {
	AuthURL     string
	UserURL     string
	CreatorURL  string
	ContentURL  string
	VideoURL    string
	MediaURL    string
	CourseURL   string
	LearningURL string
	SocialURL   string
	CommerceURL string
	PaymentURL  string
	NotificationURL string
}

func Load() Config {
	port := 8080

	if value := os.Getenv("PORT"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			port = parsed
		}
	}

	return Config{
		AppEnv: getEnv("APP_ENV", "development"),
		Port:   port,

		Services: ServicesConfig{
			AuthURL:     getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
			UserURL:     getEnv("USER_SERVICE_URL", "http://localhost:8082"),
			CreatorURL:  getEnv("CREATOR_SERVICE_URL", "http://localhost:8083"),
			ContentURL:  getEnv("CONTENT_SERVICE_URL", "http://localhost:8084"),
			VideoURL:    getEnv("VIDEO_SERVICE_URL", "http://localhost:8085"),
			MediaURL:    getEnv("MEDIA_SERVICE_URL", "http://localhost:8091"),
			CourseURL:   getEnv("COURSE_SERVICE_URL", "http://localhost:8086"),
			LearningURL: getEnv("LEARNING_SERVICE_URL", "http://localhost:8087"),
			SocialURL:   getEnv("SOCIAL_SERVICE_URL", "http://localhost:8088"),
			CommerceURL: getEnv("COMMERCE_SERVICE_URL", "http://localhost:8089"),
			PaymentURL:  getEnv("PAYMENT_SERVICE_URL", "http://localhost:8090"),
			NotificationURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8092"),
		},
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
