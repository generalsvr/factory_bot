package ai

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

type Provider struct {
	client *openai.Client
}

func NewProvider(openRouterKey string) *Provider {
	config := openai.DefaultConfig(openRouterKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	client := openai.NewClientWithConfig(config)

	return &Provider{
		client: client,
	}
}

func (p *Provider) Generate(ctx context.Context, messages []openai.ChatCompletionMessage, model string, maxTokens int) (string, error) {
	logrus.WithFields(logrus.Fields{
		"model":      model,
		"max_tokens": maxTokens,
		"msg_count":  len(messages),
	}).Info("Sending request to AI model")

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    false,
	})

	if err != nil {
		logrus.WithError(err).WithField("model", model).Error("❌ OpenRouter API request failed")
		return "", fmt.Errorf("openrouter API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		logrus.WithField("model", model).Error("❌ No response choices returned")
		return "", fmt.Errorf("no response choices returned")
	}

	content := resp.Choices[0].Message.Content

	// Success logging
	logrus.WithFields(logrus.Fields{
		"model":            model,
		"content_length":   len(content),
		"prompt_tokens":    resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":     resp.Usage.TotalTokens,
		"finish_reason":    resp.Choices[0].FinishReason,
	}).Info("✅ AI response received successfully")

	return content, nil
}

func (p *Provider) GenerateWithVision(ctx context.Context, messages []openai.ChatCompletionMessage, model string, maxTokens int) (string, error) {
	logrus.WithFields(logrus.Fields{
		"model":      model,
		"max_tokens": maxTokens,
		"msg_count":  len(messages),
	}).Info("Sending vision request to AI model")

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    false,
	})

	if err != nil {
		logrus.WithError(err).WithField("model", model).Error("❌ OpenRouter vision API request failed")
		return "", fmt.Errorf("openrouter vision API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		logrus.WithField("model", model).Error("❌ No response choices returned from vision model")
		return "", fmt.Errorf("no response choices returned from vision model")
	}

	content := resp.Choices[0].Message.Content

	logrus.WithFields(logrus.Fields{
		"model":            model,
		"content_length":   len(content),
		"prompt_tokens":    resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":     resp.Usage.TotalTokens,
		"finish_reason":    resp.Choices[0].FinishReason,
	}).Info("✅ AI vision response received successfully")

	return content, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
