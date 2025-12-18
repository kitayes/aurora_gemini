package router

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurora/internal/characters"
	"aurora/internal/config"
	"aurora/internal/gm"
	"aurora/internal/llm"
	"aurora/internal/locations"
	"aurora/internal/models"
	"aurora/internal/quests"
	"aurora/internal/scenes"
	"aurora/internal/vk"

	"github.com/SevereCloud/vksdk/v2/api"
	longpoll "github.com/SevereCloud/vksdk/v2/longpoll-bot"
)

type Deps struct {
	Config          *config.Config
	DB              *sql.DB
	Lore            interface{}
	LLM             llm.Client
	SceneService    *scenes.Service
	VK              *api.VK
	LocationService *locations.Service
}

type formBuffer struct {
	PeerID    int
	StartedAt time.Time
	Raw       strings.Builder
}

type Router struct {
	cfg          *config.Config
	db           *sql.DB
	llm          llm.Client
	scenes       *scenes.Service
	vk           *api.VK
	charService  *characters.Service
	questService *quests.Service
	gmService    *gm.Service
	locService   *locations.Service

	formMu  sync.Mutex
	formBuf map[int64]*formBuffer
}

type FormBuffer struct {
	StartedAt time.Time
	Text      strings.Builder
}

var formBuffers = make(map[int64]*FormBuffer)

func NewRouter(d Deps) *Router {
	charSvc := characters.NewService(d.DB)
	questSvc := quests.NewService(d.DB)
	gmSvc := gm.NewService(d.Config, d.SceneService, d.LLM, d.VK)
	locSvc := locations.NewService(d.DB)

	return &Router{
		cfg:          d.Config,
		db:           d.DB,
		llm:          d.LLM,
		scenes:       d.SceneService,
		vk:           d.VK,
		charService:  charSvc,
		questService: questSvc,
		gmService:    gmSvc,
		locService:   locSvc,
		formBuf:      map[int64]*formBuffer{},
	}
}

func (r *Router) send(peerID int, msg string) {
	_, err := r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   msg,
	})
	if err != nil {
		log.Printf("send error: %v", err)
	}
}

func (r *Router) formAppendIfActive(fromID, peerID int, text string) bool {
	low := strings.ToLower(strings.TrimSpace(text))
	parts := strings.Fields(low)
	if len(parts) >= 1 && parts[0] == "!анкета" {
		return false
	}

	r.formMu.Lock()
	buf, ok := r.formBuf[int64(fromID)]
	if ok && time.Since(buf.StartedAt) > 15*time.Minute {
		delete(r.formBuf, int64(fromID))
		ok = false
	}
	r.formMu.Unlock()

	if !ok {
		return false
	}
	if buf.PeerID != peerID {
		return false
	}

	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}

	r.formMu.Lock()
	buf.Raw.WriteString("\n")
	buf.Raw.WriteString(t)
	r.formMu.Unlock()
	return true
}

func (r *Router) RegisterHandlers(lp *longpoll.LongPoll) {
	lp.MessageNew(func(ctx context.Context, obj vk.Message) {
		m := obj.Message
		fromID := m.FromID
		peerID := m.PeerID
		text := strings.TrimSpace(m.Text)
		lower := strings.ToLower(text)

		log.Printf("IN MSG peer=%d from=%d text=%q", peerID, fromID, text)

		if fromID <= 0 || text == "" {
			return
		}

		// 1️⃣ Если сейчас идёт ввод анкеты — перехватываем ВСЁ
		if r.formAppendIfActive(fromID, peerID, text) {
			return
		}

		if lower == "!ping" {
			r.send(peerID, "pong")
			return
		}

		if r.gmService.IsGM(int64(fromID)) && strings.HasPrefix(lower, "!gm") {
			handled, reply := r.gmService.HandleCommand(
				ctx, int64(peerID), int64(fromID), text,
			)
			if handled && reply != "" {
				r.send(peerID, reply)
			}
			return
		}

		if r.gmService.IsGM(int64(fromID)) {
			low := strings.ToLower(strings.TrimSpace(text))
			if strings.HasPrefix(low, "лапидарий") {
				parts := strings.Fields(text)
				rest := ""
				if len(parts) > 1 {
					rest = strings.TrimSpace(strings.TrimPrefix(text, parts[0]))
					rest = strings.TrimLeft(rest, " ,:\t")
				}
				if rest != "" {
					cmd := "!gm ask " + rest
					handled, reply := r.gmService.HandleCommand(
						ctx, int64(peerID), int64(fromID), cmd,
					)
					if handled && strings.TrimSpace(reply) != "" {
						r.send(peerID, reply)
						return
					}

					r.send(peerID, "Лапидарий молчит, словно мир затаил дыхание. Попробуй задать вопрос чуть позже.")
					return
				}
			}
		}

		if strings.HasPrefix(text, "!") {
			r.handlePlayerCommand(ctx, peerID, fromID, text)
			return
		}

		if err := r.logSceneMessage(ctx, int64(fromID), text); err != nil {
			log.Printf("log scene msg error: %v", err)
		}
	})
}

func (r *Router) logSceneMessage(ctx context.Context, fromID int64, text string) error {
	sc, err := r.scenes.GetActiveScene(ctx)
	if err != nil {
		return err
	}
	msg := models.SceneMessage{
		SceneID:    sc.ID,
		SenderType: "player",
		SenderID:   fromID,
		Content:    text,
		CreatedAt:  time.Now(),
	}
	return r.scenes.AppendMessage(ctx, msg)
}

func (r *Router) handlePlayerCommand(ctx context.Context, peerID, fromID int, text string) {
	lower := strings.ToLower(strings.TrimSpace(text))

	switch {
	case strings.HasPrefix(lower, "!локация список"):
		r.handleLocationList(ctx, peerID)
	case strings.HasPrefix(lower, "!локация текущая"):
		r.handleLocationSetCurrent(ctx, peerID, text)
	case strings.HasPrefix(lower, "!локация"):
		r.handleLocationCreate(ctx, peerID, fromID, text)
	case strings.HasPrefix(lower, "!квест"):
		r.handleQuestRequest(ctx, peerID, fromID)
	case strings.HasPrefix(lower, "!совет"):
		r.handleAdviceRequest(ctx, peerID, fromID)
	case strings.HasPrefix(lower, "!статус"):
		r.handleStatusRequest(ctx, peerID, fromID)
	case strings.HasPrefix(lower, "!ход"):
		r.handleQuestProgress(ctx, peerID, fromID, text)
	case strings.HasPrefix(lower, "!бой"):
		r.handleCombatTurn(ctx, peerID, fromID, text)
	case strings.HasPrefix(lower, "!анкета пример"):
		r.handleFormExample(ctx, peerID)
	case strings.HasPrefix(lower, "!анкета пример"):
		r.handleFormExample(ctx, peerID)
	case strings.HasPrefix(lower, "!анкета"):
		if strings.Contains(lower, "отмена") {
			r.formMu.Lock()
			delete(r.formBuf, int64(fromID))
			r.formMu.Unlock()
			r.send(peerID, "Ввод анкеты отменён.")
		} else if strings.Contains(lower, "конец") {
			r.finishCharacterForm(ctx, peerID, fromID)
		} else {
			r.startOrAppendCharacterForm(ctx, peerID, fromID, text)
		}
	case strings.HasPrefix(lower, "!локация список"):
	default:
		_, err := r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": time.Now().UnixNano(),
			"message":   "Неизвестная команда. Доступно: !квест, !совет, !статус, !ход, !бой, !анкета.",
		})
		if err != nil {
			log.Printf("unknown cmd send error: %v", err)
		}
	}
}

func (r *Router) handleFormExample(ctx context.Context, peerID int) {
	example := `Пример анкеты персонажа:

!анкета
Имя: Астрид Вейр
Раса: человек
Черты: холодная, расчетливая, предана долгу

Способности:
- холодная логика
- допросы и психологическое давление
- ритуальная магия огня

Биография:
Родилась в приграничном городе. В детстве пережила нападение культа и теперь охотится на одержимых.`

	_, err := r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   example,
	})
	if err != nil {
		log.Printf("form example send error: %v", err)
	}
}

func (r *Router) startOrAppendCharacterForm(ctx context.Context, peerID, fromID int, text string) {
	r.formMu.Lock()
	buf, exists := r.formBuf[int64(fromID)]
	if !exists {
		buf = &formBuffer{PeerID: peerID, StartedAt: time.Now()}
		r.formBuf[int64(fromID)] = buf
	}
	r.formMu.Unlock()

	if !exists {
		r.send(peerID,
			"Начат ввод анкеты.\n"+
				"Отправь анкету одним или несколькими сообщениями.\n"+
				"Когда закончишь — напиши:\n!анкета конец\n"+
				"Если передумал: !анкета отмена",
		)
	}

	clean := strings.TrimSpace(strings.TrimPrefix(text, "!анкета"))
	clean = strings.TrimLeft(clean, " \t:,")
	if clean == "" {
		return
	}

	r.formMu.Lock()
	buf.Raw.WriteString("\n")
	buf.Raw.WriteString(clean)
	r.formMu.Unlock()
}

func (r *Router) normalizeCharacterForm(ctx context.Context, raw string) (*models.NormalizedCharacterForm, error) {
	prompt := llm.BuildCharacterNormalizePrompt(raw)

	reply, err := r.llm.GeneratePlain(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var form models.NormalizedCharacterForm
	if err := json.Unmarshal([]byte(reply), &form); err != nil {
		return nil, err
	}
	return &form, nil
}

func (r *Router) finishCharacterForm(ctx context.Context, peerID, fromID int) {
	r.formMu.Lock()
	buf, exists := r.formBuf[int64(fromID)]
	if exists {
		delete(r.formBuf, int64(fromID))
	}
	r.formMu.Unlock()

	if !exists {
		r.send(peerID, "Ты не начал ввод анкеты. Используй: !анкета")
		return
	}

	raw := buf.Raw.String()
	if strings.TrimSpace(raw) == "" {
		r.send(peerID, "Анкета пуста.")
		return
	}

	form, err := r.normalizeCharacterForm(ctx, raw)
	if err != nil {
		r.send(peerID, "Не удалось разобрать анкету. Проверь формат или попробуй ещё раз.")
		return
	}

	ch, err := r.charService.UpdateFromNormalizedForm(ctx, int64(fromID), form)
	if err != nil {
		r.send(peerID, "Ошибка сохранения анкеты: "+err.Error())
		return
	}

	r.send(peerID, buildWelcomeLine(ch.Name, ch.Gender))
}

type locationForm struct {
	Name string
	Desc string
	Tags string
}

func parseLocationForm(text string) locationForm {
	lines := strings.Split(text, "\n")
	f := locationForm{}

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if l == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(l), "!локация") {
			continue
		}
		parts := strings.SplitN(l, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "название", "имя":
			f.Name = val
		case "описание":
			f.Desc = val
		case "теги", "tags":
			f.Tags = val
		}
	}

	if f.Name == "" {
		rest := strings.TrimSpace(strings.TrimPrefix(text, "!локация"))
		if rest != "" && !strings.Contains(rest, "\n") {
			f.Name = rest
		}
	}

	return f
}

func parseCharacterForm(text string) characters.Form {
	lines := strings.Split(text, "\n")
	form := characters.Form{}
	var abilitiesLines []string
	var bioLines []string
	mode := "" // "", "abilities", "bio"

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if l == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(l), "!анкета") {
			continue
		}

		low := strings.ToLower(l)
		if strings.HasPrefix(low, "способности") {
			mode = "abilities"
			continue
		}
		if strings.HasPrefix(low, "биография") {
			mode = "bio"
			continue
		}

		if strings.HasPrefix(l, "-") && mode == "abilities" {
			abilitiesLines = append(abilitiesLines, strings.TrimSpace(strings.TrimPrefix(l, "-")))
			continue
		}
		if mode == "bio" {
			bioLines = append(bioLines, l)
			continue
		}

		parts := strings.SplitN(l, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "имя":
			form.Name = val
		case "раса":
			form.Race = val
		case "черты":
			form.Traits = val
		case "цель":
			form.Goal = val
		case "локация", "локация/место":
			form.LocationName = val
		}
	}

	if len(abilitiesLines) > 0 {
		form.Abilities = strings.Join(abilitiesLines, "; ")
	}
	if len(bioLines) > 0 {
		form.Bio = strings.Join(bioLines, " ")
	}

	return form
}

func parseBlock(reply, header string) string {
	i := strings.Index(reply, header)
	if i < 0 {
		return ""
	}
	s := reply[i+len(header):]
	j := strings.Index(s, "\n[")
	if j >= 0 {
		s = s[:j]
	}
	return strings.TrimSpace(s)
}

func parseNewLocationBlock(reply string) (name, desc, tags string) {
	b := parseBlock(reply, "[NEW_LOCATION]")
	if b == "" {
		return
	}
	for _, ln := range strings.Split(b, "\n") {
		parts := strings.SplitN(strings.TrimSpace(ln), ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(parts[0]))
		v := strings.TrimSpace(parts[1])
		switch k {
		case "name":
			name = v
		case "description":
			desc = v
		case "tags":
			tags = v
		}
	}
	return
}

func parseQuestLocationBlock(reply string) (locName string) {
	b := parseBlock(reply, "[QUEST_LOCATION]")
	if b == "" {
		return ""
	}
	for _, ln := range strings.Split(b, "\n") {
		parts := strings.SplitN(strings.TrimSpace(ln), ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(parts[0]))
		v := strings.TrimSpace(parts[1])
		if k == "name" {
			return v
		}
	}
	return ""
}

func (r *Router) handleCharacterForm(ctx context.Context, peerID, fromID int, text string) {
	log.Printf("FORM start peer=%d from=%d", peerID, fromID)

	form := parseCharacterForm(text)
	log.Printf("FORM parsed name=%q race=%q loc=%q abilities_len=%d bio_len=%d",
		form.Name, form.Race, form.LocationName, len(form.Abilities), len(form.Bio))

	if form.Name == "" {
		_, err := r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": time.Now().UnixNano(),
			"message":   "Не вижу имени персонажа. Пример:\n\n!анкета\nИмя: ...\nРаса: ...\nЧерты: ...\n\nСпособности:\n- ...\n\nБиография:\n...",
		})
		if err != nil {
			log.Printf("FORM missing name send error: %v", err)
		}
		return
	}

	// Локация необязательна: пытаемся взять из сцены, но если сцена падает/пустая — дефолт
	if form.LocationName == "" {
		if sc, err := r.scenes.GetActiveScene(ctx); err == nil && sc.LocationName != "" {
			form.LocationName = sc.LocationName
		} else {
			form.LocationName = "Столица Авроры"
		}
	}

	ch, err := r.charService.UpdateFromForm(ctx, int64(fromID), form)
	if err != nil {
		log.Printf("FORM save error: %v", err)
		_, sendErr := r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": time.Now().UnixNano(),
			"message":   "Не удалось сохранить анкету: " + err.Error(),
		})
		if sendErr != nil {
			log.Printf("FORM error reply send error: %v", sendErr)
		}
		return
	}

	msg := "Анкета сохранена. Персонаж: %s" + ch.Name

	_, err = r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   msg,
	})
	if err != nil {
		log.Printf("FORM ok reply send error: %v", err)
	} else {
		log.Printf("FORM ok reply sent")
	}
}

func (r *Router) handleQuestRequest(ctx context.Context, peerID, fromID int) {
	ch, err := r.charService.GetOrCreateByVK(ctx, int64(fromID))
	if err != nil {
		log.Printf("get character error: %v", err)
		return
	}
	sc, err := r.scenes.GetActiveScene(ctx)
	if err != nil {
		log.Printf("get scene error: %v", err)
		return
	}
	history, err := r.scenes.GetLastMessagesSummary(ctx, sc.ID, 10)
	if err != nil {
		log.Printf("history error: %v", err)
		return
	}
	activeQuests, err := r.questService.GetActiveForCharacter(ctx, ch.ID)
	if err != nil {
		log.Printf("quests error: %v", err)
		return
	}

	pctx := llm.PlayerContext{
		Character:     ch,
		Scene:         sc,
		History:       history,
		Quests:        activeQuests,
		LocationTag:   sc.LocationName,
		FactionTag:    ch.FactionName,
		CustomTags:    []string{"квест", "экономика"},
		PlayerMessage: "PlayerMessage: `Дай новое задание/квест для этого персонажа в контексте текущей сцены.\n\nЛокация НЕ обязательна.\n\nЕсли хочешь добавить новую локацию — добавь блок в самом конце ответа:\n\n[NEW_LOCATION]\nname: <название>\ndescription: <1-3 предложения>\ntags: <через запятую>\n\nЕсли квест привязан к существующей или новой локации — добавь:\n\n[QUEST_LOCATION]\nname: <название или пусто>`,\n",
	}

	reply, err := r.llm.GenerateForPlayer(ctx, pctx)
	log.Printf("LLM REPLY len=%d tail=%q", len(reply), lastN(reply, 80))
	if err != nil {
		log.Printf("llm error: %v", err)
		_, sendErr := r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": time.Now().UnixNano(),
			"message":   "Духи мира молчат. Попробуй ещё раз позже.",
		})
		if sendErr != nil {
			log.Printf("quest llm fail send error: %v", sendErr)
		}
		return
	}

	newLocName, newLocDesc, newLocTags := parseNewLocationBlock(reply)
	questLocName := parseQuestLocationBlock(reply)

	var locID sql.NullInt64
	var locName string

	if newLocName != "" {
		loc, err := r.locService.Create(ctx, newLocName, newLocDesc, newLocTags, "ai")
		if err == nil && loc != nil {
			locID = sql.NullInt64{Int64: loc.ID, Valid: true}
			locName = loc.Name
		}
	}

	if !locID.Valid && questLocName != "" {
		if loc, err := r.locService.GetByName(ctx, questLocName); err == nil && loc != nil {
			locID = sql.NullInt64{Int64: loc.ID, Valid: true}
			locName = loc.Name
		}
	}

	if q, err := r.questService.CreateFromAI(ctx, ch.ID, reply); err != nil {
		log.Printf("create quest error: %v", err)
	} else if q != nil {
		if locID.Valid {
			if err := r.questService.SetLocation(ctx, q.ID, locID.Int64); err != nil {
				log.Printf("set quest location error: %v", err)
			} else {
				reply += "\n\n📍 Локация квеста: " + locName
			}
		}
		reply += "\n\n(Квест добавлен в твой список.)"
	}

	if err := r.scenes.AppendMessage(ctx, models.SceneMessage{
		SceneID:    sc.ID,
		SenderType: "ai",
		SenderID:   0,
		Content:    reply,
		CreatedAt:  time.Now(),
	}); err != nil {
		log.Printf("scene log error: %v", err)
	}

	_, err = r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   reply,
	})
	if err != nil {
		log.Printf("quest send error: %v", err)
	}
	log.Printf("OUT MSG peer=%d len=%d", peerID, len(reply))
}

func (r *Router) handleAdviceRequest(ctx context.Context, peerID, fromID int) {
	ch, err := r.charService.GetOrCreateByVK(ctx, int64(fromID))
	if err != nil {
		log.Printf("get character error: %v", err)
		return
	}
	sc, err := r.scenes.GetActiveScene(ctx)
	if err != nil {
		log.Printf("get scene error: %v", err)
		return
	}
	history, err := r.scenes.GetLastMessagesSummary(ctx, sc.ID, 10)
	if err != nil {
		log.Printf("history error: %v", err)
		return
	}
	activeQuests, err := r.questService.GetActiveForCharacter(ctx, ch.ID)
	if err != nil {
		log.Printf("quests error: %v", err)
		return
	}

	pctx := llm.PlayerContext{
		Character:     ch,
		Scene:         sc,
		History:       history,
		Quests:        activeQuests,
		LocationTag:   sc.LocationName,
		FactionTag:    ch.FactionName,
		CustomTags:    []string{"совет"},
		PlayerMessage: "Подскажи, какие 1–3 варианта действий сейчас логичны для этого персонажа.",
	}

	reply, err := r.llm.GenerateForPlayer(ctx, pctx)
	if err != nil {
		log.Printf("llm error: %v", err)
		_, sendErr := r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": time.Now().UnixNano(),
			"message":   "Тени шепчут невнятно. Попробуй ещё раз.",
		})
		if sendErr != nil {
			log.Printf("advice llm fail send error: %v", sendErr)
		}
		return
	}

	if err := r.scenes.AppendMessage(ctx, models.SceneMessage{
		SceneID:    sc.ID,
		SenderType: "ai",
		SenderID:   0,
		Content:    reply,
		CreatedAt:  time.Now(),
	}); err != nil {
		log.Printf("scene log error: %v", err)
	}

	_, err = r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   reply,
	})
	if err != nil {
		log.Printf("advice send error: %v", err)
	}
	log.Printf("OUT MSG peer=%d len=%d", peerID, len(reply))
}

// ---- Статус ----

func (r *Router) handleStatusRequest(ctx context.Context, peerID, fromID int) {
	ch, err := r.charService.GetOrCreateByVK(ctx, int64(fromID))
	if err != nil {
		log.Printf("get character error: %v", err)
		return
	}
	qs, err := r.questService.GetActiveForCharacter(ctx, ch.ID)
	if err != nil {
		log.Printf("quests error: %v", err)
		return
	}
	if len(qs) == 0 {
		_, err := r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": time.Now().UnixNano(),
			"message":   "У тебя нет активных квестов.",
		})
		if err != nil {
			log.Printf("status send error: %v", err)
		}
		return
	}

	var sb strings.Builder
	sb.WriteString("Твои активные квесты:\n")
	for _, q := range qs {
		sb.WriteString("— " + q.Title + " (id " + strconv.FormatInt(q.ID, 10) + ", стадия " + strconv.Itoa(q.Stage) + ", сложность " + q.Difficulty + ")\n")
	}

	_, err = r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   sb.String(),
	})
	if err != nil {
		log.Printf("status send error: %v", err)
	}
}

func (r *Router) handleQuestProgress(ctx context.Context, peerID, fromID int, text string) {
	lines := strings.Split(strings.TrimSpace(text), "\n")

	if len(lines) < 2 {
		r.send(peerID, "Использование:\n\n!ход <id локации>\n<описание действия>")
		return
	}

	// --- парсим первую строку: "!ход <id>" ---
	header := strings.Fields(strings.TrimSpace(lines[0]))
	if len(header) < 2 {
		r.send(peerID, "Укажи id локации.\nПример:\n\n!ход 12\nЯ ищу слухи на рынке.")
		return
	}

	locID, err := strconv.ParseInt(header[1], 10, 64)
	if err != nil {
		r.send(peerID, "Неверный id локации.")
		return
	}

	// --- описание действия ---
	action := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	if action == "" {
		r.send(peerID, "Опиши действие персонажа.")
		return
	}

	// --- персонаж ---
	ch, err := r.charService.GetOrCreateByVK(ctx, int64(fromID))
	if err != nil {
		log.Printf("character error: %v", err)
		return
	}

	// --- активный квест ---
	qs, err := r.questService.GetActiveForCharacter(ctx, ch.ID)
	if err != nil {
		log.Printf("quests error: %v", err)
		r.send(peerID, "Не удалось получить активные квесты.")
		return
	}
	if len(qs) == 0 {
		r.send(peerID, "У тебя нет активных квестов. Сначала возьми: !квест")
		return
	}
	q := qs[0]

	// --- сцена ---
	sc, err := r.scenes.GetActiveScene(ctx)
	if err != nil {
		log.Printf("scene error: %v", err)
		return
	}

	// --- локация ---
	loc, err := r.locService.GetByID(ctx, locID)
	if err != nil {
		r.send(peerID, "Локация не найдена.")
		return
	}

	if err := r.scenes.SetActiveSceneLocation(
		ctx,
		sql.NullInt64{Int64: loc.ID, Valid: true},
		loc.Name,
	); err != nil {
		log.Printf("set scene location error: %v", err)
	}

	sc.LocationName = loc.Name

	// --- история ---
	history, err := r.scenes.GetLastMessagesSummary(ctx, sc.ID, 10)
	if err != nil {
		log.Printf("history error: %v", err)
		return
	}

	// --- LLM ---
	qCtx := llm.QuestProgressContext{
		Character:    ch,
		Scene:        sc,
		Quest:        q,
		History:      history,
		PlayerAction: action,
	}

	result, err := r.llm.GenerateQuestProgress(ctx, qCtx)
	if err != nil {
		log.Printf("quest progress error: %v", err)
		r.send(peerID, "Судьба квеста неясна. Попробуй ещё раз.")
		return
	}

	// --- обновления ---
	if result.Stage > 0 {
		q.Stage = result.Stage
	}
	if result.Completed {
		q.Status = "completed"
	}
	if err := r.questService.UpdateProgress(ctx, q); err != nil {
		log.Printf("quest update error: %v", err)
	}

	if result.RewardGold > 0 {
		ch.Gold += result.RewardGold
	}
	if err := r.charService.UpdateCombatAndGold(ctx, ch); err != nil {
		log.Printf("char gold update error: %v", err)
	}

	// --- ответ ---
	textOut := result.Narration
	if result.RewardGold > 0 {
		textOut += "\n\nТы получаешь " + strconv.Itoa(result.RewardGold) + " золотых."
	}
	if len(result.RewardItems) > 0 {
		textOut += "\nНаграда (предметы):"
		for _, it := range result.RewardItems {
			textOut += "\n— " + it
		}
	}

	if err := r.scenes.AppendMessage(ctx, models.SceneMessage{
		SceneID:    sc.ID,
		SenderType: "ai",
		SenderID:   0,
		Content:    textOut,
		CreatedAt:  time.Now(),
	}); err != nil {
		log.Printf("scene log error: %v", err)
	}

	_, _ = r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": time.Now().UnixNano(),
		"message":   textOut,
	})
}

func (r *Router) handleCombatTurn(ctx context.Context, peerID, fromID int, text string) {
	action := strings.TrimSpace(strings.TrimPrefix(text, "!бой"))
	if action == "" {
		_, _ = r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": 0,
			"message":   "Использование: !бой <описание твоих действий в бою>",
		})
		return
	}
	ch, err := r.charService.GetOrCreateByVK(ctx, int64(fromID))
	if err != nil {
		log.Printf("char error: %v", err)
		return
	}
	sc, err := r.scenes.GetActiveScene(ctx)
	if err != nil {
		log.Printf("scene error: %v", err)
		return
	}
	history, err := r.scenes.GetLastMessagesSummary(ctx, sc.ID, 10)
	if err != nil {
		log.Printf("history error: %v", err)
		return
	}

	// можно поискать связанный активный квест, сейчас берём первый
	var q *models.Quest
	qs, _ := r.questService.GetActiveForCharacter(ctx, ch.ID)
	if len(qs) > 0 {
		q = &qs[0]
	}

	cCtx := llm.CombatContext{
		Character:    ch,
		Scene:        sc,
		Quest:        q,
		History:      history,
		PlayerAction: action,
	}
	result, err := r.llm.GenerateCombatTurn(ctx, cCtx)
	if err != nil {
		log.Printf("combat error: %v", err)
		_, _ = r.vk.MessagesSend(api.Params{
			"peer_id":   peerID,
			"random_id": 0,
			"message":   "Боги войны молчат. Попробуй ещё раз.",
		})
		return
	}

	ch.CombatHealth = result.PlayerHP
	if ch.CombatHealth < 0 {
		ch.CombatHealth = 0
	}
	if err := r.charService.UpdateCombatAndGold(ctx, ch); err != nil {
		log.Printf("char combat update error: %v", err)
	}

	textOut := result.RoundDesc + "\n\n(Твоё здоровье: " + strconv.Itoa(result.PlayerHP) + "%)"

	if err := r.scenes.AppendMessage(ctx, models.SceneMessage{
		SceneID:    sc.ID,
		SenderType: "ai",
		SenderID:   0,
		Content:    textOut,
		CreatedAt:  time.Now(),
	}); err != nil {
		log.Printf("scene log error: %v", err)
	}

	_, _ = r.vk.MessagesSend(api.Params{
		"peer_id":   peerID,
		"random_id": 0,
		"message":   textOut,
	})
}

// ---- ЛОКАЦИИ ----

func (r *Router) handleLocationList(ctx context.Context, peerID int) {
	ls, err := r.locService.List(ctx, 20)
	if err != nil {
		log.Printf("location list error: %v", err)
		r.send(peerID, "Не удалось получить список локаций.")
		return
	}
	if len(ls) == 0 {
		r.send(peerID, "Локаций пока нет. Создай: !локация ...")
		return
	}

	var sb strings.Builder
	sb.WriteString("📍 Локации мира:\n")
	for _, l := range ls {
		sb.WriteString("— [" + strconv.FormatInt(l.ID, 10) + "] " + l.Name)
		if l.Tags != "" {
			sb.WriteString(" (" + l.Tags + ")")
		}
		sb.WriteString("\n")
	}
	r.send(peerID, sb.String())
}

func (r *Router) handleLocationCreate(ctx context.Context, peerID, fromID int, text string) {
	f := parseLocationForm(text)
	if f.Name == "" {
		r.send(peerID, "Использование:\n\n!локация\nНазвание: ...\nОписание: ...\nТеги: ...\n\nили коротко:\n!локация Название")
		return
	}

	createdBy := "gm"
	if !r.gmService.IsGM(int64(fromID)) {
		createdBy = "player"
	}

	loc, err := r.locService.Create(ctx, f.Name, f.Desc, f.Tags, createdBy)
	if err != nil {
		log.Printf("location create error: %v", err)
		r.send(peerID, "Не удалось создать локацию.")
		return
	}

	r.send(peerID, "📍 Локация создана: "+loc.Name+"\nID: "+strconv.FormatInt(loc.ID, 10))
}

func (r *Router) handleLocationSetCurrent(ctx context.Context, peerID int, text string) {
	parts := strings.Fields(text)
	if len(parts) < 3 {
		r.send(peerID, "Использование:\n!локация текущая <id>")
		return
	}

	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		r.send(peerID, "Неверный id локации.")
		return
	}

	loc, err := r.locService.GetByID(ctx, id)
	if err != nil {
		r.send(peerID, "Локация не найдена.")
		return
	}

	err = r.scenes.SetActiveSceneLocation(
		ctx,
		sql.NullInt64{Int64: loc.ID, Valid: true},
		loc.Name,
	)
	if err != nil {
		log.Printf("set scene location error: %v", err)
		r.send(peerID, "Не удалось установить текущую локацию.")
		return
	}

	r.send(peerID, "Текущая локация сцены: "+loc.Name)
}

func lastN(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func buildWelcomeLine(name, gender string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "путник"
	}

	switch strings.ToLower(gender) {
	case "женский":
		return "Анкета сохранена. Аврора приветствует свою героиню " + name + "!"
	default:
		return "Анкета сохранена. Аврора приветствует своего героя " + name + "!"
	}
}
