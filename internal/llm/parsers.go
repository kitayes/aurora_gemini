package llm

import (
	"encoding/json"
	"strings"
)

func parseQuestProgress(raw string) QuestProgressResult {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	type questJson struct {
		Stage            int      `json:"stage"`
		Completed        bool     `json:"completed"`
		Narration        string   `json:"narration"`
		RewardGold       int      `json:"reward_gold"`
		RewardItems      []string `json:"reward_items"`
		RewardReputation int      `json:"reward_reputation"`
	}

	var temp questJson
	if err := json.Unmarshal([]byte(clean), &temp); err != nil {
		return QuestProgressResult{
			Narration: raw,
		}
	}

	return QuestProgressResult{
		Stage:       temp.Stage,
		Completed:   temp.Completed,
		Narration:   temp.Narration,
		RewardGold:  temp.RewardGold,
		RewardItems: temp.RewardItems,
	}
}

func parseCombatResult(raw string) CombatResult {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	type combatJson struct {
		RoundDesc   string `json:"round_desc"`
		PlayerHP    int    `json:"player_hp"`
		EnemyHP     int    `json:"enemy_hp"`
		EnemyStatus string `json:"enemy_status"`
		Winner      string `json:"winner"`
		IsFinished  bool   `json:"is_finished"`
	}

	var temp combatJson
	if err := json.Unmarshal([]byte(clean), &temp); err != nil {
		return CombatResult{
			RoundDesc:  raw,
			PlayerHP:   -1,
			EnemyHP:    -1,
			IsFinished: false,
		}
	}

	isFinished := temp.IsFinished
	if temp.Winner != "" && temp.Winner != "none" {
		isFinished = true
	}
	if temp.PlayerHP <= 0 {
		isFinished = true
		if temp.Winner == "" {
			temp.Winner = "enemy"
		}
	}
	if temp.EnemyHP <= 0 {
		isFinished = true
		if temp.Winner == "" {
			temp.Winner = "player"
		}
	}

	return CombatResult{
		RoundDesc:   temp.RoundDesc,
		PlayerHP:    temp.PlayerHP,
		EnemyHP:     temp.EnemyHP,
		EnemyStatus: temp.EnemyStatus,
		IsFinished:  isFinished,
		Winner:      temp.Winner,
	}
}
