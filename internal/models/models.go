package models

import (
	"database/sql"
	"time"
)

type Character struct {
	ID       int64
	VKUserID int64

	Name        string
	Gender      string
	Race        string
	Class       string
	Country     string
	FactionID   int64
	FactionName string

	Traits string
	Goal   string

	LocationID   sql.NullInt64
	LocationName string

	Status string

	Abilities string
	Bio       string

	SheetJSON string

	CombatPower  int
	CombatHealth int
	Gold         int

	CreatedAt time.Time
}

type NormalizedCharacterForm struct {
	Name       string         `json:"name"`
	Gender     string         `json:"gender"`
	Race       string         `json:"race"`
	Country    string         `json:"country"`
	Meta       map[string]any `json:"meta"`
	Sheet      map[string]any `json:"sheet"`
	Abilities  []string       `json:"abilities"`
	Inventory  []string       `json:"inventory"`
	Bio        string         `json:"bio"`
	TraitsPos  []string       `json:"traits_positive"`
	TraitsNeg  []string       `json:"traits_negative"`
	Motivation string         `json:"motivation"`
}

type Scene struct {
	ID           int64
	Name         string
	LocationID   sql.NullInt64 // было int64
	LocationName string
	Summary      string
	GMMode       string
	IsActive     bool
	CreatedAt    time.Time
}
type Quest struct {
	ID          int64
	CharacterID int64
	Title       string
	Description string
	Stage       int
	Status      string
	From        string
	Difficulty  string
	RewardValue int
	LocationID  sql.NullInt64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SceneMessage struct {
	ID         int64
	SceneID    int64
	SenderType string // "player"/"ai"/"gm"
	SenderID   int64
	Content    string
	CreatedAt  time.Time
}

type Location struct {
	ID          int64
	Name        string
	Description string
	Tags        string
	CreatedBy   string
	CreatedAt   time.Time
}
