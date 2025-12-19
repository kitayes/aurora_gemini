package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurora/internal/lore"
)

type OpenAIClient struct {
	apiKey   string
	model    string
	loreRepo lore.Repository
	client   *http.Client
}

func NewOpenAIClient(apiKey, model string, loreRepo lore.Repository) *OpenAIClient {
	return &OpenAIClient{
		apiKey:   apiKey,
		model:    model,
		loreRepo: loreRepo,
		client:   &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *OpenAIClient) GeneratePlain(ctx context.Context, prompt string) (string, error) {
	msgs := []chatMessage{{Role: "user", Content: prompt}}
	return c.callChat(ctx, msgs)
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *OpenAIClient) AskLapidarius(ctx context.Context, pCtx PlayerContext, question string) (string, error) {
	return "Модуль Лапидария не поддерживается в OpenAI версии. Переключитесь на Gemini.", nil
}

func (c *OpenAIClient) Summarize(ctx context.Context, oldSummary string, newMessages []string) (string, error) {
	textBlock := strings.Join(newMessages, "\n")
	prompt := fmt.Sprintf(`
ТЫ — МОДУЛЬ СЖАТИЯ ПАМЯТИ.
Твоя задача: обновить краткое содержание (саммари) сцены, добавив в него новые события.

[ТЕКУЩЕЕ САММАРИ]:
%s

[НОВЫЕ СОБЫТИЯ]:
%s

ИНСТРУКЦИЯ:
1. Объедини старое и новое в один связный текст (до 150 слов).
2. Сохрани имена NPC, важные решения и полученные предметы.
3. Убери "воду" и пустые диалоги.
4. Пиши в прошедшем времени.

НОВОЕ САММАРИ:`, oldSummary, textBlock)

	msgs := []chatMessage{{Role: "user", Content: prompt}}
	return c.callChat(ctx, msgs)
}

func (c *OpenAIClient) callChat(ctx context.Context, messages []chatMessage) (string, error) {
	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai status %d", resp.StatusCode)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("empty openai response")
	}
	return cr.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) GenerateForPlayer(ctx context.Context, pCtx PlayerContext) (string, error) {
	systemPrompt := BuildPlayerSystemPrompt()
	baseLore := c.loreRepo.GetCoreLore()
	loreBlocks := c.loreRepo.SelectRelevant(pCtx.LocationTag, pCtx.FactionTag, pCtx.CustomTags)
	contextText := BuildPlayerContextBlock(pCtx, baseLore, loreBlocks)

	msgs := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "assistant", Content: contextText},
		{Role: "user", Content: pCtx.PlayerMessage},
	}
	return c.callChat(ctx, msgs)
}

func (c *OpenAIClient) GenerateForGM(ctx context.Context, prompt string) (string, error) {
	msgs := []chatMessage{
		{Role: "system", Content: BuildGMSystemPrompt() + "\n\n" + c.loreRepo.GetCoreLore()},
		{Role: "user", Content: prompt},
	}
	return c.callChat(ctx, msgs)
}

func (c *OpenAIClient) GenerateQuestProgress(ctx context.Context, qCtx QuestProgressContext) (QuestProgressResult, error) {
	baseLore := c.loreRepo.GetCoreLore()
	loreBlocks := c.loreRepo.SelectRelevant(qCtx.Scene.LocationName, qCtx.Character.FactionName, []string{"экономика", "квест"})
	prompt := BuildQuestProgressPrompt(qCtx, baseLore, loreBlocks)

	msgs := []chatMessage{
		{Role: "system", Content: BuildQuestSystemPrompt()},
		{Role: "user", Content: prompt},
	}
	raw, err := c.callChat(ctx, msgs)
	if err != nil {
		return QuestProgressResult{}, err
	}
	return parseQuestProgress(raw), nil
}

func (c *OpenAIClient) GenerateCombatTurn(ctx context.Context, cCtx CombatContext) (CombatResult, error) {
	baseLore := c.loreRepo.GetCoreLore()
	loreBlocks := c.loreRepo.SelectRelevant(cCtx.Scene.LocationName, cCtx.Character.FactionName, []string{"бой", "магия", "экономика"})
	prompt := BuildCombatPrompt(cCtx, baseLore, loreBlocks)

	msgs := []chatMessage{
		{Role: "system", Content: BuildCombatSystemPrompt()},
		{Role: "user", Content: prompt},
	}
	raw, err := c.callChat(ctx, msgs)
	if err != nil {
		return CombatResult{}, err
	}
	return parseCombatResult(raw), nil
}
