package models

import "time"

type Quest struct {
	ID          int64
	CharacterID int64
	LocationID  int64
	Title       string
	Description string
	From        string
	Status      string
	Stage       int
	Difficulty  string
	RewardGold  int
	RewardItem  string
	RewardValue int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}