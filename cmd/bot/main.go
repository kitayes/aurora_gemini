package main

import (
	"log"
	"os"

	"aurora/internal/config"
	"aurora/internal/llm"
	"aurora/internal/lore"
	"aurora/internal/router"
	"aurora/internal/scenes"
	"aurora/internal/storage"
	"aurora/internal/vk"
)

func main() {
	log.Println("Starting Aurora bot...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := storage.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("migrations error: %v", err)
	}

	loreRepo, err := lore.NewFileLoreRepo("lore")
	if err != nil {
		log.Fatalf("lore load error: %v", err)
	}

	var llmClient llm.Client
	switch cfg.LLMProvider {
	case "openai":
		llmClient = llm.NewOpenAIClient(cfg.OpenAIKey, cfg.LLMModel, loreRepo)
	default:
		llmClient = llm.NewGeminiClient(cfg.GeminiKey, cfg.LLMModel, loreRepo)
	}

	sceneService := scenes.NewService(db)
	if err := sceneService.EnsureDefaultScene(); err != nil {
		log.Fatalf("scene init error: %v", err)
	}

	vkAPI := vk.NewVK(cfg.VKToken)
	lp, err := vk.NewLongPoll(vkAPI, cfg.VKGroupID)
	if err != nil {
		log.Fatalf("vk longpoll error: %v", err)
	}

	rt := router.NewRouter(router.Deps{
		Config:       cfg,
		DB:           db,
		Lore:         loreRepo,
		LLM:          llmClient,
		SceneService: sceneService,
		VK:           vkAPI,
	})

	rt.RegisterHandlers(lp)

	log.Println("Aurora bot is running...")
	if err := lp.Run(); err != nil {
		log.Printf("longpoll stopped: %v", err)
		os.Exit(1)
	}
}
