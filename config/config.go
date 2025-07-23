package config

import "os"

type Config struct {
	OpenRouterKey string
	BotToken      string
	TextModel     string
	VisionModel   string
}

func Load() *Config {
	textModel := os.Getenv("TEXT_MODEL")
	if textModel == "" {
		textModel = "google/gemini-2.5-pro"
	}

	visionModel := os.Getenv("VISION_MODEL")
	if visionModel == "" {
		visionModel = "google/gemini-2.5-pro"
	}

	return &Config{
		OpenRouterKey: os.Getenv("OPENROUTER_KEY"),
		BotToken:      os.Getenv("BOT_TOKEN"),
		TextModel:     textModel,
		VisionModel:   visionModel,
	}
}
