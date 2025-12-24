package llm

import (
	"context"
	"encoding/json"
	"strings"
)

type IntentType string

const (
	IntentChat          IntentType = "CHAT"
	IntentUseItem       IntentType = "USE_ITEM"
	IntentEquip         IntentType = "EQUIP"
	IntentQuestDecision IntentType = "QUEST_DECISION"
	IntentGM            IntentType = "GM_COMMAND"
)

type IntentResult struct {
	Type   IntentType `json:"type"`
	Target string     `json:"target"`
}

func (c *GeminiClient) ClassifyIntent(ctx context.Context, text string, isGM bool) (IntentResult, error) {
	prompt := `
Твоя задача — классифицировать сообщение игрока в текстовой RPG.
Верни JSON.

ДОСТУПНЫЕ ТИПЫ (IntentType):
1. "USE_ITEM": Игрок хочет съесть, выпить, прочитать или использовать предмет.
   Target: название предмета.
2. "EQUIP": Игрок хочет взять в руки, надеть броню или оружие.
   Target: название предмета.
3. "QUEST_DECISION": Игрок явно соглашается или отказывается от предложения.
   Target: "accept" или "decline".
4. "GM_COMMAND": (Только если message выглядит как админская просьба: "обнули здоровье", "дай меч").
   Target: описание просьбы.
5. "CHAT": Всё остальное (вопросы, описание действий, болтовня).

ПРИМЕРЫ:
- "Выпей зелье лечения" -> {"type": "USE_ITEM", "target": "зелье лечения"}
- "Достань меч" -> {"type": "EQUIP", "target": "меч"}
- "Я согласен на это задание" -> {"type": "QUEST_DECISION", "target": "accept"}
- "Нет, это слишком опасно" -> {"type": "QUEST_DECISION", "target": "decline"}
- "Привет, Лапидарий" -> {"type": "CHAT", "target": ""}
- "Атакую орка" -> {"type": "CHAT", "target": ""} (Боевые действия идут через !бой, тут это просто чат)

СООБЩЕНИЕ: "` + text + `"`

	if isGM {
		prompt += "\n(Пользователь — админ/ГМ)."
	}

	resp, err := c.GeneratePlain(ctx, prompt)
	if err != nil {
		return IntentResult{Type: IntentChat}, nil
	}

	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimSuffix(clean, "```")

	var res IntentResult
	if err := json.Unmarshal([]byte(clean), &res); err != nil {
		return IntentResult{Type: IntentChat}, nil
	}
	return res, nil
}
