package llm

import (
	"strconv"
	"strings"
)

func parseQuestProgress(raw string) QuestProgressResult {
	res := QuestProgressResult{
		Stage:      0,
		Completed:  false,
		RewardGold: 0,
	}

	section := ""
	lines := strings.Split(raw, "\n")
	var narrationLines []string

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if l == "" {
			continue
		}
		switch {
		case strings.HasPrefix(l, "[STAGE]:"):
			v := strings.TrimSpace(strings.TrimPrefix(l, "[STAGE]:"))
			if n, err := strconv.Atoi(v); err == nil {
				res.Stage = n
			}
			section = ""
		case strings.HasPrefix(l, "[COMPLETED]:"):
			v := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(l, "[COMPLETED]:")))
			res.Completed = (v == "да" || v == "yes" || v == "true")
			section = ""
		case strings.HasPrefix(l, "[NARRATION]:"):
			section = "narration"
			narrationLines = append(narrationLines, strings.TrimSpace(strings.TrimPrefix(l, "[NARRATION]:")))
		case strings.HasPrefix(l, "[REWARD_GOLD]:"):
			v := strings.TrimSpace(strings.TrimPrefix(l, "[REWARD_GOLD]:"))
			if n, err := strconv.Atoi(v); err == nil {
				res.RewardGold = n
			}
			section = ""
		case strings.HasPrefix(l, "[REWARD_ITEMS]:"):
			section = "items"
		case strings.HasPrefix(l, "[REWARD_REPUTATION]:"):
			section = "reputation"
		case strings.HasPrefix(l, "-"):
			item := strings.TrimSpace(strings.TrimPrefix(l, "-"))
			if item == "" {
				continue
			}
			if section == "items" {
				res.RewardItems = append(res.RewardItems, item)
			} else if section == "reputation" {
				res.RewardReputation = append(res.RewardReputation, item)
			}
		default:
			if section == "narration" {
				narrationLines = append(narrationLines, l)
			}
		}
	}

	if len(narrationLines) == 0 {
		res.Narration = strings.TrimSpace(raw)
	} else {
		res.Narration = strings.TrimSpace(strings.Join(narrationLines, "\n"))
	}
	return res
}

func parseCombatResult(raw string) CombatResult {
	cr := CombatResult{
		PlayerHP: 100,
		EnemyHP:  100,
		Status:   "ongoing",
	}

	section := ""
	lines := strings.Split(raw, "\n")
	var roundLines []string

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if l == "" {
			continue
		}
		switch {
		case strings.HasPrefix(l, "[ROUND_DESC]:"):
			section = "round"
			roundLines = append(roundLines, strings.TrimSpace(strings.TrimPrefix(l, "[ROUND_DESC]:")))
		case strings.HasPrefix(l, "[PLAYER_STATE]:"):
			section = "player"
		case strings.HasPrefix(l, "[ENEMY_STATE]:"):
			section = "enemy"
		case strings.HasPrefix(l, "[COMBAT_STATUS]:"):
			v := strings.TrimSpace(strings.TrimPrefix(l, "[COMBAT_STATUS]:"))
			cr.Status = strings.ToLower(v)
			section = ""
		case strings.HasPrefix(strings.ToLower(l), "здоровье:"):
			// здоровье: <n>
			parts := strings.SplitN(l, ":", 2)
			if len(parts) == 2 {
				nStr := strings.Fields(strings.TrimSpace(parts[1]))
				if len(nStr) > 0 {
					if n, err := strconv.Atoi(nStr[0]); err == nil {
						if section == "player" {
							cr.PlayerHP = n
						} else if section == "enemy" {
							cr.EnemyHP = n
						}
					}
				}
			}
		case strings.HasPrefix(strings.ToLower(l), "состояние:"):
			parts := strings.SplitN(l, ":", 2)
			if len(parts) == 2 {
				st := strings.TrimSpace(parts[1])
				if section == "player" {
					cr.PlayerState = st
				} else if section == "enemy" {
					cr.EnemyState = st
				}
			}
		default:
			if section == "round" {
				roundLines = append(roundLines, l)
			}
		}
	}

	if len(roundLines) == 0 {
		cr.RoundDesc = strings.TrimSpace(raw)
	} else {
		cr.RoundDesc = strings.TrimSpace(strings.Join(roundLines, "\n"))
	}
	return cr
}
