package vk

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"aurora/internal/application"
	"aurora/internal/llm"
	"aurora/internal/models"
	"aurora/pkg/config"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/events"
	longpoll "github.com/SevereCloud/vksdk/v2/longpoll-bot"
)

type formBuffer struct {
	PeerID    int
	StartedAt time.Time
	Raw       strings.Builder
}

type Handler struct {
	cfg          *config.Config
	vk           *api.VK
	llm          llm.Client
	charService  *application.CharacterService
	questService *application.QuestService
	sceneService *application.SceneService
	locService   *application.LocationService
	gmService    *application.GMService

	formMu  sync.Mutex
	formBuf map[int64]*formBuffer
}

func NewHandler(
	cfg *config.Config,
	vk *api.VK,
	llm llm.Client,
	charService *application.CharacterService,
	questService *application.QuestService,
	sceneService *application.SceneService,
	locService *application.LocationService,
	gmService *application.GMService,
) *Handler {
	return &Handler{
		cfg:          cfg,
		vk:           vk,
		llm:          llm,
		charService:  charService,
		questService: questService,
		sceneService: sceneService,
		locService:   locService,
		gmService:    gmService,
		formBuf:      make(map[int64]*formBuffer),
	}
}

func (h *Handler) send(peerID int, msg string) {
	_, err := h.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   msg,
	})
	if err != nil {
		log.Printf("send error: %v", err)
	}
}

func (h *Handler) Start(lp *longpoll.LongPoll) {
	lp.MessageNew(func(ctx context.Context, obj events.MessageNewObject) {
		m := obj.Message
		fromID := m.FromID
		peerID := m.PeerID
		text := strings.TrimSpace(m.Text)
		lower := strings.ToLower(text)

		log.Printf("IN MSG peer=%d from=%d text=%q", peerID, fromID, text)

		if fromID <= 0 || text == "" {
			return
		}

		if h.formAppendIfActive(fromID, peerID, text) {
			return
		}

		if lower == "!ping" {
			h.send(peerID, "pong")
			return
		}

		if h.gmService.IsGM(int64(fromID)) && strings.HasPrefix(lower, "!gm") {
			handled, reply := h.gmService.HandleCommand(ctx, int64(peerID), int64(fromID), text)
			if handled && reply != "" {
				h.send(peerID, reply)
			}
			return
		}

		if strings.HasPrefix(text, "!") {
			if !strings.HasPrefix(lower, "!лапидарий") {
				h.handlePlayerCommand(ctx, peerID, fromID, text)
				return
			}
		}

		// 2. --- ПРОВЕРКА ОБРАЩЕНИЯ (СТРОГИЙ ФИЛЬТР) ---
		isTriggerPhrase := strings.HasPrefix(lower, "!лапидарий") ||
			strings.Contains(lower, "сфера лапидария")

		isReplyToBot := m.ReplyMessage != nil && m.ReplyMessage.FromID < 0

		if isTriggerPhrase || isReplyToBot {
			isGM := h.gmService.IsGM(int64(fromID))

			// Спрашиваем ИИ о намерениях
			intent, err := h.llm.ClassifyIntent(ctx, text, isGM)
			if err != nil {
				intent = llm.IntentResult{Type: llm.IntentChat}
			}

			switch intent.Type {
			case llm.IntentUseItem:
				// h.handleUseItem(ctx, peerID, fromID, intent.Target)
				h.send(peerID, "Использование предметов пока не реализовано в новой архитектуре.")
				return
			default:
				h.handleLapidariusChat(ctx, peerID, fromID, text)
				return
			}
		}

		// 3. --- ЛОГИРОВАНИЕ ИСТОРИИ (БЕСПЛАТНО) ---
		isMainChat := h.cfg.RPPeerID == 0 || peerID == h.cfg.RPPeerID

		if isMainChat {
			if len(text) > 5 && !strings.HasPrefix(text, "((") {
				if err := h.logSceneMessage(ctx, int64(fromID), text); err != nil {
					log.Printf("log scene msg error: %v", err)
				}
			}
		}
	})
}

func (h *Handler) formAppendIfActive(fromID, peerID int, text string) bool {
	low := strings.ToLower(strings.TrimSpace(text))
	parts := strings.Fields(low)
	if len(parts) >= 1 && parts[0] == "!анкета" {
		return false
	}

	h.formMu.Lock()
	buf, ok := h.formBuf[int64(fromID)]
	if ok && time.Since(buf.StartedAt) > 15*time.Minute {
		delete(h.formBuf, int64(fromID))
		ok = false
	}
	h.formMu.Unlock()

	if !ok {
		return false
	}
	if buf.PeerID != peerID {
		return false
	}

	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}

	h.formMu.Lock()
	buf.Raw.WriteString("\n")
	buf.Raw.WriteString(t)
	h.formMu.Unlock()
	return true
}

func (h *Handler) handlePlayerCommand(ctx context.Context, peerID, fromID int, text string) {
	lower := strings.ToLower(strings.TrimSpace(text))

	switch {
	case strings.HasPrefix(lower, "!принимаю"):
		h.handleQuestDecision(ctx, peerID, fromID, "accept")
	case strings.HasPrefix(lower, "!отказываюсь"):
		h.handleQuestDecision(ctx, peerID, fromID, "decline")
	case strings.HasPrefix(lower, "!сюжет") || strings.HasPrefix(lower, "!хроника"):
		h.handleSummaryRequest(ctx, peerID, fromID)
	case strings.HasPrefix(lower, "!квест"):
		h.handleQuestRequest(ctx, peerID, fromID)
	case strings.HasPrefix(lower, "!анкета пример"):
		// h.handleFormExample(ctx, peerID)
	case strings.HasPrefix(lower, "!анкета"):
		if strings.Contains(lower, "отмена") {
			h.formMu.Lock()
			delete(h.formBuf, int64(fromID))
			h.formMu.Unlock()
			h.send(peerID, "Ввод анкеты отменён.")
		} else if strings.Contains(lower, "конец") {
			h.finishCharacterForm(ctx, peerID, fromID)
		} else {
			h.startOrAppendCharacterForm(ctx, peerID, fromID, text)
		}
	default:
		h.send(peerID, "Неизвестная команда. Доступно: !квест, !принимаю, !отказываюсь, !анкета, !сюжет.")
	}
}

func (h *Handler) handleLapidariusChat(ctx context.Context, peerID, fromID int, text string) {
	question := text
	lower := strings.ToLower(text)

	if strings.HasPrefix(lower, "!лапидарий") {
		question = strings.TrimPrefix(text, "!лапидарий")
		question = strings.TrimPrefix(question, "!Лапидарий")
	} else if idx := strings.Index(lower, "сфера лапидария"); idx != -1 {
		part1 := text[:idx]
		part2 := text[idx+len("сфера лапидария"):]
		question = part1 + part2
	}
	question = strings.TrimSpace(strings.TrimLeft(question, " ,.!?:"))

	if question == "" {
		h.send(peerID, "Сфера тихо гудит. Ей нужен вопрос.")
		return
	}

	ch, err := h.charService.GetOrCreateByVK(ctx, int64(fromID))
	if err != nil {
		h.send(peerID, "Сфера не видит твою ауру.")
		return
	}

	sc, err := h.sceneService.GetOrCreateSceneForCharacter(ctx, ch.ID)
	if err != nil {
		sc = models.Scene{Name: "Ошибка мира", LocationName: "Пустота"}
	}

	history, _ := h.sceneService.GetLastMessagesSummary(ctx, sc.ID, 5)
	qs, _ := h.questService.GetActiveForCharacter(ctx, ch.ID)

	pCtx := llm.PlayerContext{
		Character:     *ch,
		Scene:         sc,
		History:       history,
		Quests:        qs,
		LocationTag:   sc.LocationName,
		FactionTag:    ch.FactionName,
		PlayerMessage: question,
		CustomTags:    []string{"лор", "совет"},
	}

	answer, err := h.llm.AskLapidarius(ctx, pCtx, question)
	if err != nil {
		log.Printf("Lapidarius error: %v", err)
		h.send(peerID, "Сфера пошла трещинами (Ошибка магии).")
		return
	}

	h.send(peerID, answer)
}

func (h *Handler) logSceneMessage(ctx context.Context, fromID int64, text string) error {
	ch, err := h.charService.GetOrCreateByVK(ctx, fromID)
	if err != nil {
		return err
	}
	sc, err := h.sceneService.GetOrCreateSceneForCharacter(ctx, ch.ID)
	if err != nil {
		return err
	}

	return h.sceneService.AppendMessage(ctx, models.SceneMessage{
		SceneID:    sc.ID,
		SenderType: "player",
		SenderID:   ch.ID,
		Content:    text,
		CreatedAt:  time.Now(),
	})
}
