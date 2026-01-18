package llm

const (
	MaxQuestGold = 500

	MaxCombatGold = 50

	MaxItemsPerQuest = 3

	MaxDamagePerHit = 50

	MinDamagePerHit = 5

	MaxHealingPerTurn = 30

	MaxItemValue = 200
)

func ValidateQuestReward(result *QuestProgressResult) {
	if result.RewardGold > MaxQuestGold {
		result.RewardGold = MaxQuestGold
	}
	if result.RewardGold < 0 {
		result.RewardGold = 0
	}

	if len(result.RewardItems) > MaxItemsPerQuest {
		result.RewardItems = result.RewardItems[:MaxItemsPerQuest]
	}
}

func ValidateCombatResult(result *CombatResult, originalPlayerHP, originalEnemyHP int) {
	damageTaken := originalPlayerHP - result.PlayerHP
	if damageTaken > MaxDamagePerHit {
		result.PlayerHP = originalPlayerHP - MaxDamagePerHit
	}

	damageDealt := originalEnemyHP - result.EnemyHP
	if damageDealt > MaxDamagePerHit {
		result.EnemyHP = originalEnemyHP - MaxDamagePerHit
	}

	if result.PlayerHP > 100 {
		result.PlayerHP = 100
	}

	if result.PlayerHP < 0 {
		result.PlayerHP = 0
	}
	if result.EnemyHP < 0 {
		result.EnemyHP = 0
	}
}

func CalculateAppropriateReward(difficulty string, baseValue int) int {
	multipliers := map[string]float64{
		"trivial": 0.1,
		"easy":    0.25,
		"normal":  0.5,
		"hard":    0.75,
		"deadly":  1.0,
		"epic":    1.0, 
	}

	mult, ok := multipliers[difficulty]
	if !ok {
		mult = 0.5
	}

	reward := int(float64(baseValue) * mult)
	if reward > MaxQuestGold {
		reward = MaxQuestGold
	}

	return reward
}

var ItemRarityLimits = map[string]int{
	"common":    50,
	"uncommon":  100,
	"rare":      200,
	"epic":      MaxItemValue,
	"legendary": MaxItemValue,
}
