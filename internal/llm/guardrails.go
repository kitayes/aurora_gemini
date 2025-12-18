package llm

import (
	"strings"
	"unicode"
)

type GuardrailsConfig struct {
	MinWordsLore  int
	MinWordsFight int
	EnableLLMFix  bool
}

func GuessIsAction(playerInput string) bool {
	s := strings.ToLower(playerInput)
	keywords := []string{"бой", "атака", "удар", "меч", "стрела", "кров", "ран", "уклон", "взрыв", "погон"}
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func WordCount(s string) int {
	return len(strings.Fields(s))
}

func HasQuestionMark(s string) bool {
	return strings.Contains(s, "?")
}

func EndsWithChoiceOrQuestion(s string) bool {
	tail := s
	if len(tail) > 300 {
		tail = tail[len(tail)-300:]
	}
	tailLower := strings.ToLower(tail)

	if strings.Contains(tailLower, "?") {
		return true
	}
	choiceMarkers := []string{
		"что вы сделаете дальше",
		"что вы делаете дальше",
		"ваш выбор",
		"как вы поступите",
		"выберите",
		"что дальше",
	}
	for _, m := range choiceMarkers {
		if strings.Contains(tailLower, m) {
			return true
		}
	}
	return false
}

func LooksPastTenseRU(s string) bool {
	low := strings.ToLower(s)
	anchors := []string{"был", "была", "были", "стал", "стала", "стали", "сказал", "сказала", "ответил", "ответила", "шёл", "шла", "вошёл", "вошла"}
	for _, a := range anchors {
		if strings.Contains(low, a) {
			return true
		}
	}
	return false
}

func EnsureEndingChoice(s string) string {
	if EndsWithChoiceOrQuestion(s) {
		return s
	}

	add := "\n\n**Выбор:** что вы сделали дальше — рискнули и пошли вперёд, попытались договориться, или отступили, чтобы перегруппироваться?"
	return strings.TrimRight(s, "\n ") + add
}

func TrimWeirdTail(s string) string {
	return strings.TrimRightFunc(s, func(r rune) bool {
		return unicode.IsSpace(r)
	})
}

type Validation struct {
	IsAction          bool
	Words             int
	MinWordsRequired  int
	HasEndingChoice   bool
	LooksPastTense    bool
	NeedsHardFixByLLM bool
}

func ValidateGMReply(cfg GuardrailsConfig, playerInput, reply string) Validation {
	isAction := GuessIsAction(playerInput)
	minWords := cfg.MinWordsLore
	if isAction {
		minWords = cfg.MinWordsFight
		if minWords == 0 {
			minWords = 140
		}
	} else {
		if minWords == 0 {
			minWords = 320
		}
	}

	words := WordCount(reply)
	hasEnding := EndsWithChoiceOrQuestion(reply)
	past := LooksPastTenseRU(reply)

	needsHard := false
	if words < minWords || !hasEnding || !past {
		needsHard = true
	}

	return Validation{
		IsAction:          isAction,
		Words:             words,
		MinWordsRequired:  minWords,
		HasEndingChoice:   hasEnding,
		LooksPastTense:    past,
		NeedsHardFixByLLM: needsHard && cfg.EnableLLMFix,
	}
}

func BuildRepairPrompt(systemPrompt, context, playerInput, badReply string, v Validation) string {
	lengthHint := "соблюдай объём"
	if v.IsAction {
		lengthHint = "сделай 200–300 слов, короткие энергичные абзацы"
	} else {
		lengthHint = "сделай 500–800 слов, атмосферно (звуки/запахи/свет)"
	}

	return systemPrompt + "\n\n" +
		"=== КАНОНИЧНЫЙ КОНТЕКСТ (НЕ ИЗМЕНЯТЬ) ===\n" + context + "\n\n" +
		"=== ДЕЙСТВИЯ ИГРОКОВ ===\n" + playerInput + "\n\n" +
		"=== ТВОЙ ПРЕДЫДУЩИЙ ОТВЕТ (НУЖЕН РЕМОНТ) ===\n" + badReply + "\n\n" +
		"ЗАДАЧА: перепиши ответ ГМа, НЕ меняя событий/фактов, но строго по правилам:\n" +
		"- прошедшее время, третье лицо\n" +
		"- Markdown: **имена**, *мысли*, > речь NPC\n" +
		"- " + lengthHint + "\n" +
		"- в конце ОБЯЗАТЕЛЬНО ситуация выбора или открытый вопрос\n" +
		"Верни только финальный исправленный пост."
}
