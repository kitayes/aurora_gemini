package models

import "time"

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