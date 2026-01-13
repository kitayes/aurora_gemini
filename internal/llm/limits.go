package llm

// Жёсткие лимиты экономики, которые НЕ могут быть превышены AI
const (
	// Максимальное золото за квест (даже epic)
	MaxQuestGold = 500

	// Максимальное золото за победу в бою
	MaxCombatGold = 50

	// Максимум предметов за один квест
	MaxItemsPerQuest = 3

	// Максимальный урон за один удар
	MaxDamagePerHit = 50

	// Минимальный урон от любой атаки (нет "промахов" без причины)
	MinDamagePerHit = 5

	// Максимальное исцеление за ход
	MaxHealingPerTurn = 30

	// Максимальная стоимость одного предмета в награде
	MaxItemValue = 200
)

// ValidateQuestReward проверяет и ограничивает награду квеста
func ValidateQuestReward(result *QuestProgressResult) {
	// Ограничение золота
	if result.RewardGold > MaxQuestGold {
		result.RewardGold = MaxQuestGold
	}
	if result.RewardGold < 0 {
		result.RewardGold = 0
	}

	// Ограничение количества предметов
	if len(result.RewardItems) > MaxItemsPerQuest {
		result.RewardItems = result.RewardItems[:MaxItemsPerQuest]
	}
}

// ValidateCombatResult проверяет и ограничивает результат боя
func ValidateCombatResult(result *CombatResult, originalPlayerHP, originalEnemyHP int) {
	// Ограничение урона по игроку
	damageTaken := originalPlayerHP - result.PlayerHP
	if damageTaken > MaxDamagePerHit {
		result.PlayerHP = originalPlayerHP - MaxDamagePerHit
	}

	// Ограничение урона по врагу
	damageDealt := originalEnemyHP - result.EnemyHP
	if damageDealt > MaxDamagePerHit {
		result.EnemyHP = originalEnemyHP - MaxDamagePerHit
	}

	// Игрок не может лечиться выше 100
	if result.PlayerHP > 100 {
		result.PlayerHP = 100
	}

	// HP не может быть отрицательным
	if result.PlayerHP < 0 {
		result.PlayerHP = 0
	}
	if result.EnemyHP < 0 {
		result.EnemyHP = 0
	}
}

// CalculateAppropriateReward рассчитывает адекватную награду по сложности
func CalculateAppropriateReward(difficulty string, baseValue int) int {
	multipliers := map[string]float64{
		"trivial": 0.1,
		"easy":    0.25,
		"normal":  0.5,
		"hard":    0.75,
		"deadly":  1.0,
		"epic":    1.0, // Даже epic не может превысить лимит
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

// ItemRarityLimits определяет лимиты по редкости предметов
var ItemRarityLimits = map[string]int{
	"common":    50,
	"uncommon":  100,
	"rare":      200,
	"epic":      MaxItemValue,
	"legendary": MaxItemValue, // Легендарные предметы — только через специальные квесты
}
