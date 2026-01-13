package models

import (
	"database/sql"
	"time"
)

const (
	HealthExcellent = 90
	HealthGood      = 70
	HealthWounded   = 50
	HealthCritical  = 25
	HealthAlive     = 0
)

type Character struct {
	ID           int64
	VKUserID     int64
	Name         string
	Race         string
	Class        string
	FactionID    int64
	FactionName  string
	Traits       string
	Goal         string
	LocationID   int64
	LocationName string
	Status       string
	Abilities    string
	Bio          string
	CombatPower  int
	CombatHealth int
	Gold         int
	Gender       string
	Country      string
	SheetJSON    string
	Inventory    string
	CreatedAt    time.Time

	Effects []Effect
}

type Effect struct {
	ID          int64
	CharacterID int64
	Name        string
	Description string
	Duration    int
	IsHidden    bool
}

type Location struct {
	ID          int64
	Name        string
	Description string
	Tags        string
	IsActive    bool
	CreatedBy   string
	CreatedAt   time.Time
}

type Scene struct {
	ID           int64
	CharacterID  int64
	LocationID   sql.NullInt64
	LocationName string
	Name         string
	Status       string
	Summary      string
	Context      string
	GMMode       bool
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SceneMessage struct {
	ID         int64
	SceneID    int64
	SenderType string
	SenderID   int64
	Content    string
	CreatedAt  time.Time
}

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

type NormalizedCharacterForm struct {
	Name       string   `json:"name"`
	Surname    string   `json:"surname"`
	Nickname   string   `json:"nickname"`
	Gender     string   `json:"gender"`
	Age        int      `json:"age"`
	Race       string   `json:"race"`
	Class      string   `json:"class"`
	Country    string   `json:"country"`
	City       string   `json:"city"`
	Faction    string   `json:"faction"`
	Occupation string   `json:"occupation"`
	TraitsPos  []string `json:"traits_positive"`
	TraitsNeg  []string `json:"traits_negative"`
	Motivation string   `json:"motivation"`
	Fears      []string `json:"fears"`
	Abilities  []string `json:"abilities"`
	Skills     []string `json:"skills"`
	Inventory  []string `json:"inventory"`
	Bio        string   `json:"bio"`
	Appearance string   `json:"appearance"`
}

func (c *Character) GetStatusDescription() string {
	hp := c.CombatHealth

	switch {
	case hp >= HealthExcellent:
		return "Ты полон сил и готов к свершениям."
	case hp >= HealthGood:
		return "Ты немного утомлен, на теле пара царапин."
	case hp >= HealthWounded:
		return "Ты ранен. Движения даются тяжелее, дыхание сбито."
	case hp >= HealthCritical:
		return "Ты тяжело ранен! Кровь заливает глаза, каждое движение причиняет боль."
	case hp > HealthAlive:
		return "ТЫ ПРИ СМЕРТИ. Мир плывет перед глазами, жизнь висит на волоске."
	default:
		return "Твое тело бездыханно."
	}
}
