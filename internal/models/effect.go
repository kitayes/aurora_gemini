package models

type Effect struct {
	ID          int64
	CharacterID int64
	Name        string
	Description string
	Duration    int
	IsHidden    bool
}