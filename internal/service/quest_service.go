package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"aurora/internal/models"
	"aurora/internal/repository"
)

type QuestService struct {
	repo *repository.QuestRepository
}

func NewQuestService(repo *repository.QuestRepository) *QuestService {
	return &QuestService{repo: repo}
}

func (s *QuestService) GetActiveForCharacter(ctx context.Context, charID int64) ([]models.Quest, error) {
	return s.repo.GetActiveForCharacter(ctx, charID)
}

func (s *QuestService) GetByID(ctx context.Context, id int64) (models.Quest, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *QuestService) UpdateProgress(ctx context.Context, q models.Quest) error {
	q.UpdatedAt = time.Now()
	return s.repo.Update(ctx, q)
}

func (s *QuestService) CreateFromAI(ctx context.Context, charID int64, raw string) (*models.Quest, error) {
	lines := strings.Split(raw, "\n")
	var title, desc, qtype, qdiff string
	var qvalue int

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(l, "[QUEST_TITLE]:"):
			title = strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_TITLE]:"))
		case strings.HasPrefix(l, "[QUEST_DESCRIPTION]:"):
			desc = strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_DESCRIPTION]:"))
		case strings.HasPrefix(l, "[QUEST_TYPE]:"):
			qtype = strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_TYPE]:"))
		case strings.HasPrefix(l, "[QUEST_DIFFICULTY]:"):
			qdiff = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_DIFFICULTY]:")))
		case strings.HasPrefix(l, "[QUEST_VALUE]:"):
			vStr := strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_VALUE]:"))
			if v, err := strconv.Atoi(vStr); err == nil {
				qvalue = v
			}
		}
	}
	if title == "" {
		return nil, nil // Не нашли квест в ответе
	}
	if qtype != "" {
		desc += "\n(Тип: " + qtype + ")"
	}
	if qdiff == "" {
		qdiff = "normal"
	}
	if qvalue <= 0 {
		qvalue = 100
	}

	now := time.Now()
	q := &models.Quest{
		CharacterID: charID,
		Title:       title,
		Description: desc,
		Stage:       1,
		Status:      "active",
		From:        "ai",
		Difficulty:  qdiff,
		RewardValue: qvalue,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := s.repo.Create(ctx, q)
	if err != nil {
		return nil, err
	}
	q.ID = id
	return q, nil
}

func (s *QuestService) SetLocation(ctx context.Context, questID, locID int64) error {
	q, err := s.repo.GetByID(ctx, questID)
	if err != nil {
		return err
	}
	q.LocationID = locID
	return s.repo.Update(ctx, q)
}
