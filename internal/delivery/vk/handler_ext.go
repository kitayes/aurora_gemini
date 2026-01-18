package vk

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"aurora/internal/llm"
	"aurora/internal/models"
)

func (h *Handler) handleQuestRequest(ctx context.Context, peerID, fromID int) {
	ch, err := h.charService.GetOrCreateByVK(ctx, int64(fromID))
	if err != nil {
		return
	}

	active, err := h.questService.GetActiveForCharacter(ctx, ch.ID)
	if len(active) > 0 {
		h.send(peerID, "У тебя уже есть активный квест.")
		return
	}

	sc, err := h.sceneService.GetOrCreateSceneForCharacter(ctx, ch.ID)
	if err != nil {
		return
	}
	history, _ := h.sceneService.GetLastMessagesSummary(ctx, sc.ID, 10)

	pctx := llm.PlayerContext{
		Character:     *ch,
		Scene:         sc,
		History:       history,
		Quests:        active,
		LocationTag:   sc.LocationName,
		FactionTag:    ch.FactionName,
		CustomTags:    []string{"квест", "экономика"},
		PlayerMessage: "PlayerMessage: `Дай новое задание...`",
	}

	reply, err := h.llm.GenerateForPlayer(ctx, pctx)
	if err != nil {
		h.send(peerID, "Духи молчат.")
		return
	}

	if q, err := h.questService.CreateFromAI(ctx, ch.ID, reply); err == nil && q != nil {
		reply += "\n\nСоздан квест: " + q.Title
	}

	h.send(peerID, reply)
}

func (h *Handler) handleQuestDecision(_ context.Context, peerID, _ int, decision string) {
	h.send(peerID, "Решения по квестам: "+decision)
}

func (h *Handler) handleSummaryRequest(_ context.Context, peerID, _ int) {
	h.send(peerID, "Саммари пока не подключено.")
}

func (h *Handler) startOrAppendCharacterForm(ctx context.Context, peerID, fromID int, text string) {
	h.formMu.Lock()
	buf, exists := h.formBuf[int64(fromID)]
	if !exists {
		buf = &formBuffer{PeerID: peerID, StartedAt: time.Now()}
		h.formBuf[int64(fromID)] = buf
		h.send(peerID, "Начат ввод анкеты. Пиши и в конце: !анкета конец")
	}
	h.formMu.Unlock()

	clean := strings.TrimSpace(strings.TrimPrefix(text, "!анкета"))
	if clean != "" {
		h.formMu.Lock()
		buf.Raw.WriteString("\n" + clean)
		h.formMu.Unlock()
	}
}

func (h *Handler) finishCharacterForm(ctx context.Context, peerID, fromID int) {
	h.formMu.Lock()
	buf, exists := h.formBuf[int64(fromID)]
	if exists {
		delete(h.formBuf, int64(fromID))
	}
	h.formMu.Unlock()

	if !exists {
		return
	}

	raw := buf.Raw.String()
	form, err := h.normalizeCharacterForm(ctx, raw)
	if err != nil {
		h.send(peerID, "Ошибка анкеты")
		return
	}

	ch, err := h.charService.UpdateFromNormalizedForm(ctx, int64(fromID), form)
	if err == nil {
		h.send(peerID, "Персонаж обновлен: "+ch.Name)
	}
}

func (h *Handler) normalizeCharacterForm(ctx context.Context, raw string) (*models.NormalizedCharacterForm, error) {
	prompt := llm.BuildCharacterNormalizePrompt(raw)
	reply, err := h.llm.GeneratePlain(ctx, prompt)
	if err != nil {
		return nil, err
	}

	clean := strings.TrimSpace(reply)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")

	var form models.NormalizedCharacterForm
	if err := json.Unmarshal([]byte(clean), &form); err != nil {
		return nil, err
	}
	return &form, nil
}
