package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	VKToken   string
	VKGroupID int
	// LLMProvider: "gemini" or "openai"
	RPPeerID    int `envconfig:"RP_PEER_ID" default:"0"`
	LLMProvider string
	OpenAIKey   string
	GeminiKey   string
	LLMModel    string
	DBPath      string
	GMUserID    int 
}

func Load() (*Config, error) {
	get := func(key string) string { return os.Getenv(key) }

	vkToken := get("VK_TOKEN")
	group := get("VK_GROUP_ID")
	provider := strings.ToLower(strings.TrimSpace(get("LLM_PROVIDER")))
	if provider == "" {
		provider = "gemini"
	}

	openAIKey := get("OPENAI_API_KEY")
	geminiKey := get("GEMINI_API_KEY")

	llmModel := get("LLM_MODEL")
	if llmModel == "" {
		if provider == "openai" {
			llmModel = "gpt-4.1"
		} else {
			llmModel = "gemini-2.5-flash"
		}
	}
	dbPath := get("DB_PATH")
	if dbPath == "" {
		dbPath = "aurora.db"
	}
	gmIDStr := get("GM_USER_ID")

	rpPeerIDStr := get("RP_PEER_ID")

	if vkToken == "" || group == "" {
		return nil, fmt.Errorf("VK_TOKEN and VK_GROUP_ID are required")
	}
	if provider == "openai" {
		if openAIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required when LLM_PROVIDER=openai")
		}
	} else {
		if geminiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is required when LLM_PROVIDER=gemini")
		}
	}

	groupID, err := strconv.Atoi(group)
	if err != nil {
		return nil, fmt.Errorf("invalid VK_GROUP_ID: %w", err)
	}

	var gmID int
	if gmIDStr != "" {
		gmID, err = strconv.Atoi(gmIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid GM_USER_ID: %w", err)
		}
	}

	var rpPeerID int
	if rpPeerIDStr != "" {
		rpPeerID, err = strconv.Atoi(rpPeerIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid RP_PEER_ID: %w", err)
		}
	}

	return &Config{
		VKToken:     vkToken,
		VKGroupID:   groupID,
		LLMProvider: provider,
		OpenAIKey:   openAIKey,
		GeminiKey:   geminiKey,
		LLMModel:    llmModel,
		DBPath:      dbPath,
		GMUserID:    gmID,
		RPPeerID:    rpPeerID,
	}, nil
}
