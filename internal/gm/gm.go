package gm

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"aurora/internal/config"
	"aurora/internal/llm"
	"aurora/internal/scenes"
	"github.com/SevereCloud/vksdk/v2/api"
)

type Service struct {
	cfg    *config.Config
	scenes *scenes.Service
	llm    llm.Client
	vk     *api.VK
}

func NewService(cfg *config.Config, scenes *scenes.Service, llm llm.Client, vk *api.VK) *Service {
	return &Service{cfg: cfg, scenes: scenes, llm: llm, vk: vk}
}

func (s *Service) IsGM(vkUserID int64) bool {
	return s.cfg.GMUserID != 0 && int(vkUserID) == s.cfg.GMUserID
}

func (s *Service) HandleCommand(ctx context.Context, peerID int64, fromID int64, text string) (bool, string) {
	if !strings.HasPrefix(text, "!gm") {
		return false, ""
	}
	fields := strings.Fields(text)
	if len(fields) == 1 {
		return true, "Команды: !gm mode <human|ai_assist|ai_full>, !gm ask <вопрос>, !gm say <текст>, !gm setgm <vk_id>."
	}
	cmd := fields[1]

	switch cmd {
	case "mode":
		if len(fields) < 3 {
			return true, "Использование: !gm mode <human|ai_assist|ai_full>"
		}
		mode := fields[2]
		sc, err := s.scenes.GetActiveScene(ctx)
		if err != nil {
			return true, "Ошибка сцены: " + err.Error()
		}
		if err := s.scenes.SetGMMode(ctx, sc.ID, mode); err != nil {
			return true, "Ошибка режима: " + err.Error()
		}
		return true, fmt.Sprintf("Режим ведущего: %s", mode)

	case "ask":
		if len(fields) < 3 {
			return true, "Использование: !gm ask <вопрос>"
		}
		q := strings.TrimSpace(strings.TrimPrefix(text, "!gm ask"))
		reply, err := s.llm.GenerateForGM(ctx, q)
		if err != nil {
			return true, "Ошибка ИИ-помощника: " + err.Error()
		}
		return true, reply

	case "say":
		if len(fields) < 3 {
			return true, "Использование: !gm say <текст>"
		}
		msg := strings.TrimSpace(strings.TrimPrefix(text, "!gm say"))
		return true, msg

	case "setgm":
		if len(fields) < 3 {
			return true, "Использование: !gm setgm <vk_id>"
		}
		id, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return true, "Неверный vk_id."
		}
		s.cfg.GMUserID = int(id)
		return true, fmt.Sprintf("GM_USER_ID установлен на %d", id)
	}
	return true, "Неизвестная команда GM."
}
