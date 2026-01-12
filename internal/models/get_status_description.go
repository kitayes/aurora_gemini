package models

func (c *Character) GetStatusDescription() string {
	hp := c.CombatHealth

	switch {
	case hp >= 90:
		return "Ты полон сил и готов к свершениям."
	case hp >= 70:
		return "Ты немного утомлен, на теле пара царапин."
	case hp >= 50:
		return "Ты ранен. Движения даются тяжелее, дыхание сбито."
	case hp >= 25:
		return "Ты тяжело ранен! Кровь заливает глаза, каждое движение причиняет боль."
	case hp > 0:
		return "ТЫ ПРИ СМЕРТИ. Мир плывет перед глазами, жизнь висит на волоске."
	default:
		return "Твое тело бездыханно."
	}
}
