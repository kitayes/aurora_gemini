package models

import "time"

type Location struct {
	ID          int64
	Name        string
	Description string
	Tags        string
	IsActive    bool
	CreatedBy   string
	CreatedAt   time.Time
}