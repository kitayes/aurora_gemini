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
		model = "gemini-2.5-flash"
	}
	return &GeminiClient{
		apiKey:          apiKey,
		model:           model,
		loreRepo:        loreRepo,
		client:          &http.Client{Timeout: 25 * time.Second},
		temperature:     0.9,
		topP:            0.95,
		maxOutputTokens: 1800,
	}
}

func (c *GeminiClient) GeneratePlain(ctx context.Context, prompt string) (string, error) {
	return c.callGenerateContent(ctx, prompt)
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

func (c *GeminiClient) callGenerateContent(ctx context.Context, prompt string) (string, error) {
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
	reqBody.GenerationConfig.Temperature = c.temperature
	reqBody.GenerationConfig.TopP = c.topP
	reqBody.GenerationConfig.MaxOutputTokens = c.maxOutputTokens

	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", url.PathEscape(c.model))
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

	if len(gr.Candidates) > 0 {
		log.Printf("GEMINI finishReason=%q parts=%d", gr.Candidates[0].FinishReason, len(gr.Candidates[0].Content.Parts))
	}
	if gr.UsageMetadata != nil {
		log.Printf("GEMINI tokens prompt=%d cand=%d total=%d",
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

	return c.callGenerateContent(ctx, prompt)
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

	reply, err := c.callGenerateContent(ctx, full)
	if err != nil {
		return "", err
	}

	reply = TrimWeirdTail(reply)

	reply = EnsureEndingChoice(reply)

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

		fixed, fixErr := c.callGenerateContent(ctx, repairPrompt)
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

	raw, err := c.callGenerateContent(ctx, full)
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

	raw, err := c.callGenerateContent(ctx, full)
	if err != nil {
		return CombatResult{}, err
	}
	return parseCombatResult(raw), nil
}
