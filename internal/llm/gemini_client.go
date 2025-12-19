package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurora/internal/lore"
)

const (
	// ModelFast: Анкеты, проверки, быстрые ответы Лапидария
	ModelFast = "gemini-2.5-flash-lite"
	// ModelSmart: Бой, квесты, обычные диалоги (баланс)
	ModelSmart = "gemini-2.5-flash"
	// ModelPro: Гейм-мастер, сложный сюжет, важные решения
	ModelPro = "gemini-3.0-pro-preview"
)

type GeminiClient struct {
	apiKey   string
	model    string
	loreRepo lore.Repository
	client   *http.Client

	temperature     float64
	topP            float64
	maxOutputTokens int
}

func NewGeminiClient(apiKey, model string, loreRepo lore.Repository) *GeminiClient {
	if model == "" {
		model = ModelSmart
	}
	return &GeminiClient{
		apiKey:          apiKey,
		model:           model,
		loreRepo:        loreRepo,
		client:          &http.Client{Timeout: 90 * time.Second},
		temperature:     0.9,
		topP:            0.95,
		maxOutputTokens: 8192,
	}
}

type geminiRequest struct {
	Contents []struct {
		Role  string `json:"role,omitempty"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		Temperature     float64 `json:"temperature,omitempty"`
		TopP            float64 `json:"topP,omitempty"`
		MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	} `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (c *GeminiClient) callGenerateContent(ctx context.Context, prompt string, opts *GenOptions) (string, error) {
	var reqBody geminiRequest
	reqBody.Contents = append(reqBody.Contents, struct {
		Role  string `json:"role,omitempty"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{
		Role: "user",
		Parts: []struct {
			Text string `json:"text"`
		}{{Text: prompt}},
	})

	currentModel := c.model
	temp := c.temperature
	maxTokens := c.maxOutputTokens

	if opts != nil {
		if opts.Model != "" {
			currentModel = opts.Model
		}
		if opts.Temperature > 0 {
			temp = opts.Temperature
		}
		if opts.MaxTokens > 0 {
			maxTokens = opts.MaxTokens
		}
	}

	reqBody.GenerationConfig.Temperature = temp
	reqBody.GenerationConfig.TopP = c.topP
	reqBody.GenerationConfig.MaxOutputTokens = maxTokens

	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", url.PathEscape(currentModel))
	u := endpoint + "?key=" + url.QueryEscape(c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	bodyBytes, _ := io.ReadAll(httpResp.Body)

	var gr geminiResponse
	_ = json.Unmarshal(bodyBytes, &gr)

	if gr.UsageMetadata != nil {
		log.Printf("[%s] tokens prompt=%d cand=%d total=%d",
			currentModel,
			gr.UsageMetadata.PromptTokenCount,
			gr.UsageMetadata.CandidatesTokenCount,
			gr.UsageMetadata.TotalTokenCount,
		)
	}

	if httpResp.StatusCode >= 300 {
		if gr.Error != nil {
			return "", fmt.Errorf("gemini status %d: %s (%s)", httpResp.StatusCode, gr.Error.Message, gr.Error.Status)
		}
		return "", fmt.Errorf("gemini status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	if len(gr.Candidates) == 0 {
		return "", fmt.Errorf("empty gemini response: %s", strings.TrimSpace(string(bodyBytes)))
	}
	if len(gr.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty gemini parts: %s", strings.TrimSpace(string(bodyBytes)))
	}

	var sb strings.Builder
	for _, p := range gr.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	return sb.String(), nil
}

func (c *GeminiClient) GeneratePlain(ctx context.Context, prompt string) (string, error) {
	opts := &GenOptions{
		Model:       ModelFast,
		Temperature: 0.1,
		MaxTokens:   4000,
	}
	return c.callGenerateContent(ctx, prompt, opts)
}

func (c *GeminiClient) GenerateForPlayer(ctx context.Context, pCtx PlayerContext) (string, error) {
	systemPrompt := BuildPlayerSystemPrompt()
	baseLore := c.loreRepo.GetCoreLore()
	loreBlocks := c.loreRepo.SelectRelevant(pCtx.LocationTag, pCtx.FactionTag, pCtx.CustomTags)
	contextText := BuildPlayerContextBlock(pCtx, baseLore, loreBlocks)

	prompt := strings.Join([]string{
		"ТЫ ВСЕГДА ДЕЙСТВУЕШЬ ПО СЛЕДУЮЩИМ ПРАВИЛАМ. ИХ НЕЛЬЗЯ ИГНОРИРОВАТЬ.",
		systemPrompt,
		"\n[КОНТЕКСТ]\n" + contextText,
		"\n[СООБЩЕНИЕ ИГРОКА]\n" + pCtx.PlayerMessage,
	}, "\n\n")

	opts := &GenOptions{Model: ModelSmart}
	return c.callGenerateContent(ctx, prompt, opts)
}

func (c *GeminiClient) GenerateForGM(ctx context.Context, prompt string) (string, error) {
	contextText := strings.Join([]string{
		"[БАЗОВЫЙ ЛОР МИРА]\n" + c.loreRepo.GetCoreLore(),
	}, "\n\n")

	full := strings.Join([]string{
		"ТЫ ВСЕГДА ДЕЙСТВУЕШЬ ПО СЛЕДУЮЩИМ ПРАВИЛАМ. ИХ НЕЛЬЗЯ ИГНОРИРОВАТЬ.",
		BuildGMSystemPrompt(),
		contextText,
		"[ДЕЙСТВИЯ ИГРОКОВ]\n" + prompt,
	}, "\n\n")

	full += "\n\nПОМНИ: ты ГМ. Прошедшее время. Жёсткий реализм. В конце — выбор или открытый вопрос."

	opts := &GenOptions{
		Model:       ModelPro,
		Temperature: 0.9,
		MaxTokens:   8192,
	}

	reply, err := c.callGenerateContent(ctx, full, opts)
	if err != nil {
		return "", err
	}

	reply = TrimWeirdTail(reply)
	reply = EnsureEndingChoice(reply)

	// Guardrails (валидацию) лучше тоже делать через Smart или Pro, но с низкой температурой
	cfg := GuardrailsConfig{
		MinWordsLore:  320,
		MinWordsFight: 140,
		EnableLLMFix:  true,
	}

	validation := ValidateGMReply(cfg, prompt, reply)

	if validation.NeedsHardFixByLLM {
		repairPrompt := BuildRepairPrompt(
			BuildGMSystemPrompt(),
			contextText,
			prompt,
			reply,
			validation,
		)
		fixOpts := &GenOptions{Model: ModelSmart, Temperature: 0.3}
		fixed, fixErr := c.callGenerateContent(ctx, repairPrompt, fixOpts)

		if fixErr == nil && strings.TrimSpace(fixed) != "" {
			reply = TrimWeirdTail(fixed)
			reply = EnsureEndingChoice(reply)
		}
	}

	return reply, nil
}

func (c *GeminiClient) GenerateQuestProgress(ctx context.Context, qCtx QuestProgressContext) (QuestProgressResult, error) {
	baseLore := c.loreRepo.GetCoreLore()
	loreBlocks := c.loreRepo.SelectRelevant(qCtx.Scene.LocationName, qCtx.Character.FactionName, []string{"экономика", "квест"})
	prompt := BuildQuestProgressPrompt(qCtx, baseLore, loreBlocks)

	full := strings.Join([]string{
		"ТЫ ВСЕГДА ДЕЙСТВУЕШЬ ПО СЛЕДУЮЩИМ ПРАВИЛАМ. ИХ НЕЛЬЗЯ ИГНОРИРОВАТЬ.",
		BuildQuestSystemPrompt(),
		prompt,
	}, "\n\n")

	opts := &GenOptions{
		Model:       ModelSmart,
		Temperature: 0.4,
	}

	raw, err := c.callGenerateContent(ctx, full, opts)
	if err != nil {
		return QuestProgressResult{}, err
	}
	return parseQuestProgress(raw), nil
}

func (c *GeminiClient) GenerateCombatTurn(ctx context.Context, cCtx CombatContext) (CombatResult, error) {
	baseLore := c.loreRepo.GetCoreLore()
	loreBlocks := c.loreRepo.SelectRelevant(cCtx.Scene.LocationName, cCtx.Character.FactionName, []string{"бой", "магия", "экономика"})
	prompt := BuildCombatPrompt(cCtx, baseLore, loreBlocks)

	full := strings.Join([]string{
		"ТЫ ВСЕГДА ДЕЙСТВУЕШЬ ПО СЛЕДУЮЩИМ ПРАВИЛАМ. ИХ НЕЛЬЗЯ ИГНОРИРОВАТЬ.",
		BuildCombatSystemPrompt(),
		prompt,
	}, "\n\n")

	opts := &GenOptions{
		Model:       ModelSmart,
		Temperature: 0.4,
	}

	raw, err := c.callGenerateContent(ctx, full, opts)
	if err != nil {
		return CombatResult{}, err
	}
	return parseCombatResult(raw), nil
}

func (c *GeminiClient) AskLapidarius(ctx context.Context, pCtx PlayerContext, question string) (string, error) {
	baseLore := c.loreRepo.GetCoreLore()
	searchTags := append(pCtx.CustomTags, question)
	loreBlocks := c.loreRepo.SelectRelevant(pCtx.LocationTag, pCtx.FactionTag, searchTags)
	contextBlock := BuildPlayerContextBlock(pCtx, baseLore, loreBlocks)

	fullPrompt := strings.Join([]string{
		BuildLapidariusSystemPrompt(),
		"\n=== ЧТО ВИДИТ СФЕРА (КОНТЕКСТ) ===",
		contextBlock,
		"\n=== ВОПРОС ПЕРСОНАЖА ===",
		fmt.Sprintf("Герой спрашивает: \"%s\"", question),
	}, "\n\n")

	opts := &GenOptions{
		Model:       ModelFast,
		Temperature: 0.9,
	}

	return c.callGenerateContent(ctx, fullPrompt, opts)
}

func (c *GeminiClient) Summarize(ctx context.Context, oldSummary string, newMessages []string) (string, error) {
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

	opts := &GenOptions{
		Model:       ModelFast,
		Temperature: 0.2,
	}

	return c.callGenerateContent(ctx, prompt, opts)
}
