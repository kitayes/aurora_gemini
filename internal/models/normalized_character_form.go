package models

type NormalizedCharacterForm struct {
	Name       string   `json:"name"`
	Surname    string   `json:"surname"`
	Nickname   string   `json:"nickname"`
	Gender     string   `json:"gender"`
	Age        int      `json:"age"`
	Race       string   `json:"race"`
	Class      string   `json:"class"`
	Country    string   `json:"country"`
	City       string   `json:"city"`
	Faction    string   `json:"faction"`
	Occupation string   `json:"occupation"`
	TraitsPos  []string `json:"traits_positive"`
	TraitsNeg  []string `json:"traits_negative"`
	Motivation string   `json:"motivation"`
	Fears      []string `json:"fears"`
	Abilities  []string `json:"abilities"`
	Skills     []string `json:"skills"`
	Inventory  []string `json:"inventory"`
	Bio        string   `json:"bio"`
	Appearance string   `json:"appearance"`
}