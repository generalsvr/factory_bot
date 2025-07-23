package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"factory_bot/bot"
	"factory_bot/config"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found")
	}

	// Load configuration
	cfg := config.Load()

	// Validate required configuration
	if cfg.BotToken == "" {
		log.Fatal("BOT_TOKEN is required")
	}
	if cfg.OpenRouterKey == "" {
		log.Fatal("OPENROUTER_KEY is required")
	}

	// Configure logging
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Initialize bot
	botInstance, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	// Start bot in goroutine
	go func() {
		if err := botInstance.Start(); err != nil {
			log.Fatalf("Failed to start bot: %v", err)
		}
	}()

	logrus.Info("Sector Prom Factory Bot is running. Press CTRL+C to exit.")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logrus.Info("Shutting down bot...")
}
