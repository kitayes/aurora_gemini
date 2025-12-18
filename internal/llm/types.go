package llm

import (
	"context"

	"aurora/internal/models"
)

type Client interface {
	GenerateForPlayer(ctx context.Context, pCtx PlayerContext) (string, error)
	GenerateForGM(ctx context.Context, prompt string) (string, error)
	GenerateQuestProgress(ctx context.Context, qCtx QuestProgressContext) (QuestProgressResult, error)
	GenerateCombatTurn(ctx context.Context, cCtx CombatContext) (CombatResult, error)

	GeneratePlain(ctx context.Context, prompt string) (string, error)
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
	Stage            int
	Completed        bool
	Narration        string
	RewardGold       int
	RewardItems      []string
	RewardReputation []string
}

type CombatContext struct {
	Character    models.Character
	Scene        models.Scene
	Quest        *models.Quest // может быть nil
	History      string
	PlayerAction string
}

type CombatResult struct {
	RoundDesc   string
	PlayerHP    int
	PlayerState string
	EnemyHP     int
	EnemyState  string
	Status      string // ongoing/player_win/player_lose/retreat/stalemate
}
