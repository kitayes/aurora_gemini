package llm

import (
	"context"

	"aurora/internal/models"
)

type Client interface {
	GeneratePlain(ctx context.Context, prompt string) (string, error)
	GenerateForPlayer(ctx context.Context, pCtx PlayerContext) (string, error)
	GenerateForGM(ctx context.Context, prompt string) (string, error)
	GenerateQuestProgress(ctx context.Context, qCtx QuestProgressContext) (QuestProgressResult, error)
	GenerateCombatTurn(ctx context.Context, cCtx CombatContext) (CombatResult, error)
	AskLapidarius(ctx context.Context, pCtx PlayerContext, question string) (string, error)
	Summarize(ctx context.Context, oldSummary string, newMessages []string) (string, error)

	ClassifyIntent(ctx context.Context, text string, isGM bool) (IntentResult, error)
}

type GenOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

type PlayerContext struct {
	Character     models.Character
	Scene         models.Scene
	History       string
	Quests        []models.Quest
	LocationTag   string
	FactionTag    string
	CustomTags    []string
	PlayerMessage string
}

type QuestProgressContext struct {
	Character    models.Character
	Scene        models.Scene
	Quest        models.Quest
	History      string
	PlayerAction string
}

type QuestProgressResult struct {
	Stage       int
	Completed   bool
	Narration   string
	RewardGold  int
	RewardItems []string
}

type CombatContext struct {
	Character    models.Character
	Scene        models.Scene
	Quest        *models.Quest
	History      string
	PlayerAction string
}

type CombatResult struct {
	RoundDesc   string
	PlayerHP    int
	EnemyHP     int
	EnemyStatus string
	IsFinished  bool
	Winner      string
}
