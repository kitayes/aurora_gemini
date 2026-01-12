package models

import "time"

type SceneMessage struct {
	ID         int64
	SceneID    int64
	SenderType string
	SenderID   int64
	Content    string
	CreatedAt  time.Time
}
