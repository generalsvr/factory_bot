package bot

import (
	"context"
	"fmt"
	"strings"

	"factory_bot/ai"
	"factory_bot/config"
	"factory_bot/database"
	"factory_bot/instructions"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

type Bot struct {
	api        *tgbotapi.BotAPI
	config     *config.Config
	db         *database.Database
	aiProvider *ai.Provider
}

func New(cfg *config.Config) (*Bot, error) {
	// Initialize Telegram Bot API
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	// Initialize database
	db, err := database.New("./data/bot.db")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Clear chat history on startup
	err = db.ClearAllChatHistory()
	if err != nil {
		logrus.WithError(err).Warn("Failed to clear chat history on startup")
	} else {
		logrus.Info("Chat history cleared on startup")
	}

	// Initialize AI provider
	aiProvider := ai.NewProvider(cfg.OpenRouterKey)

	return &Bot{
		api:        bot,
		config:     cfg,
		db:         db,
		aiProvider: aiProvider,
	}, nil
}

func (b *Bot) Start() error {
	b.api.Debug = false
	logrus.Infof("Bot authorized: %s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		go b.handleMessage(update.Message)
	}

	return nil
}

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	userID := message.From.ID
	username := message.From.UserName
	text := message.Text

	// Store user and message in database
	err := b.db.AddUser(userID, username, message.From.FirstName, message.From.LastName)
	if err != nil {
		logrus.WithError(err).Error("Failed to store user")
	}

	err = b.db.SaveMessage(userID, username, text, "user")
	if err != nil {
		logrus.WithError(err).Error("Failed to store message")
	}

	// Handle commands
	if strings.HasPrefix(text, "/") {
		b.handleCommand(message)
		return
	}

	// Handle photo messages
	if len(message.Photo) > 0 {
		b.handlePhoto(message)
		return
	}

	// Process regular text message
	b.processUserMessage(message)
}

func (b *Bot) handleCommand(message *tgbotapi.Message) {
	cmd := strings.Split(message.Text, " ")[0]

	switch cmd {
	case "/start":
		b.sendMessage(message.Chat.ID, instructions.InitMessageEN)
	default:
		b.sendMessage(message.Chat.ID, "Неизвестная команда. / Unknown command.")
	}
}

func (b *Bot) handlePhoto(message *tgbotapi.Message) {
	ctx := context.Background()
	userID := message.Chat.ID

	// Send typing indicator
	typing := tgbotapi.NewChatAction(userID, tgbotapi.ChatTyping)
	b.api.Send(typing)

	// Save image message to database
	imageText := func() string {
		if message.Caption != "" {
			return "[Изображение] " + message.Caption
		}
		return "[Изображение без описания]"
	}()

	err := b.db.SaveMessage(userID, message.From.UserName, imageText, "user")
	if err != nil {
		logrus.WithError(err).Error("Failed to store image message")
	}

	// Get the largest photo
	photo := message.Photo[len(message.Photo)-1]

	// Get file URL
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
	if err != nil {
		logrus.WithError(err).Error("Failed to get photo file")
		b.sendMessage(userID, "❌ Ошибка обработки изображения / Error processing image")
		return
	}

	fileURL := file.Link(b.api.Token)

	// Prepare messages for AI with image
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: instructions.MainInstructions + "\n\n" + instructions.ImageInstruction,
		},
		{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: func() string {
						if message.Caption != "" {
							return message.Caption
						}
						return "Проанализируй это изображение для производства / Analyze this image for factory operations"
					}(),
				},
				{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    fileURL,
						Detail: openai.ImageURLDetailLow,
					},
				},
			},
		},
	}

	response, err := b.aiProvider.GenerateWithVision(ctx, messages, b.config.VisionModel, 1500)
	if err != nil {
		logrus.WithError(err).Error("Failed to process image")
		b.sendMessage(userID, "❌ Ошибка анализа изображения / Error analyzing image")
		return
	}

	// Save bot response to database
	err = b.db.SaveMessage(userID, b.api.Self.UserName, response, "assistant")
	if err != nil {
		logrus.WithError(err).Error("Failed to save bot response")
	}

	b.sendMessage(userID, response)
}

func (b *Bot) processUserMessage(message *tgbotapi.Message) {
	ctx := context.Background()
	userID := message.Chat.ID
	text := message.Text

	// Send typing indicator
	typing := tgbotapi.NewChatAction(userID, tgbotapi.ChatTyping)
	b.api.Send(typing)

	// Get chat history (20 messages max)
	history, err := b.db.GetChatHistory(userID, 20)
	if err != nil {
		logrus.WithError(err).Error("Failed to get chat history")
	}

	// Prepare messages for AI with history
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: instructions.MainInstructions,
		},
	}

	// Add chat history
	for _, msg := range history {
		if msg.Role == "user" {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: msg.Text,
			})
		} else if msg.Role == "assistant" {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: msg.Text,
			})
		}
	}

	// Add current message
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	})

	response, err := b.aiProvider.Generate(ctx, messages, b.config.TextModel, 1024)
	if err != nil {
		logrus.WithError(err).Error("Failed to process message")
		b.sendMessage(userID, "❌ Ошибка обработки запроса / Error processing request")
		return
	}

	// Save bot response to database
	err = b.db.SaveMessage(userID, b.api.Self.UserName, response, "assistant")
	if err != nil {
		logrus.WithError(err).Error("Failed to save bot response")
	}

	b.sendMessage(userID, response)
}

func (b *Bot) sendMessage(chatID int64, text string) *tgbotapi.Message {
	const maxMessageLength = 4096

	if len(text) <= maxMessageLength {
		msg := tgbotapi.NewMessage(chatID, text)
		// Try markdown first, fallback to plain text
		msg.ParseMode = tgbotapi.ModeMarkdown

		sent, err := b.api.Send(msg)
		if err != nil {
			// Retry without markdown if parsing fails
			logrus.WithError(err).Warn("Markdown parsing failed, retrying as plain text")
			msg.ParseMode = ""
			sent, err = b.api.Send(msg)
			if err != nil {
				logrus.WithError(err).Error("Failed to send message")
				return nil
			}
		}

		return &sent
	}

	// Split long message
	parts := b.splitMessage(text, maxMessageLength)
	var lastMsg *tgbotapi.Message

	for i, part := range parts {
		msg := tgbotapi.NewMessage(chatID, part)
		// Try markdown first, fallback to plain text
		msg.ParseMode = tgbotapi.ModeMarkdown

		sent, err := b.api.Send(msg)
		if err != nil {
			// Retry without markdown if parsing fails
			logrus.WithError(err).WithField("part", i+1).Warn("Markdown parsing failed for part, retrying as plain text")
			msg.ParseMode = ""
			sent, err = b.api.Send(msg)
			if err != nil {
				logrus.WithError(err).WithField("part", i+1).Error("Failed to send message part")
				continue
			}
		}

		lastMsg = &sent
	}

	return lastMsg
}

func (b *Bot) splitMessage(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var parts []string
	paragraphs := strings.Split(text, "\n\n")
	var currentPart strings.Builder

	for i, paragraph := range paragraphs {
		nextLength := currentPart.Len()
		if i > 0 && currentPart.Len() > 0 {
			nextLength += 2
		}
		nextLength += len(paragraph)

		if nextLength > maxLength && currentPart.Len() > 0 {
			parts = append(parts, strings.TrimSpace(currentPart.String()))
			currentPart.Reset()
		}

		if len(paragraph) > maxLength {
			if currentPart.Len() > 0 {
				parts = append(parts, strings.TrimSpace(currentPart.String()))
				currentPart.Reset()
			}

			sentences := b.splitBySentences(paragraph, maxLength)
			for j, sentence := range sentences {
				if j == len(sentences)-1 {
					currentPart.WriteString(sentence)
				} else {
					parts = append(parts, strings.TrimSpace(sentence))
				}
			}
		} else {
			if currentPart.Len() > 0 {
				currentPart.WriteString("\n\n")
			}
			currentPart.WriteString(paragraph)
		}
	}

	if currentPart.Len() > 0 {
		parts = append(parts, strings.TrimSpace(currentPart.String()))
	}

	return parts
}

func (b *Bot) splitBySentences(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var parts []string
	sentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})

	var currentPart strings.Builder

	for i, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		if i < len(sentences)-1 {
			sentence += ". "
		}

		if currentPart.Len()+len(sentence) > maxLength && currentPart.Len() > 0 {
			parts = append(parts, strings.TrimSpace(currentPart.String()))
			currentPart.Reset()
		}

		currentPart.WriteString(sentence)
	}

	if currentPart.Len() > 0 {
		parts = append(parts, strings.TrimSpace(currentPart.String()))
	}

	return parts
}

func (b *Bot) Close() error {
	return b.db.Close()
}
