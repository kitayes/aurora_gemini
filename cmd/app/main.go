package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"aurora/internal/application"
	"aurora/internal/delivery/vk"
	"aurora/internal/embeddings"
	"aurora/internal/llm"
	"aurora/internal/lore"
	"aurora/internal/rag"
	"aurora/internal/repository"
	"aurora/pkg/config"

	"github.com/SevereCloud/vksdk/v2/api"
	longpoll "github.com/SevereCloud/vksdk/v2/longpoll-bot"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := repository.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	charRepo := repository.NewCharacterRepository(db)
	questRepo := repository.NewQuestRepository(db)
	sceneRepo := repository.NewSceneRepository(db)
	locRepo := repository.NewLocationRepository(db)

	loreRepo, err := lore.NewFileLoreRepo("lore")
	if err != nil {
		log.Printf("failed to init lore: %v (continuing with empty)", err)
	}

	var llmClient llm.Client
	if cfg.LLMProvider == "openai" {
		log.Fatal("OpenAI not implemented yet")
	} else {
		llmClient = llm.NewGeminiClient(cfg.GeminiKey, cfg.LLMModel, loreRepo)
	}

	embedder := embeddings.NewGeminiEmbedder(cfg.GeminiKey)
	vectorRepo := repository.NewVectorRepository(db)
	ragService := rag.NewService(embedder, vectorRepo, loreRepo)

	if geminiClient, ok := llmClient.(*llm.GeminiClient); ok {
		geminiClient.SetRAGService(ragService)
		log.Println("✅ RAG service enabled for semantic search")
	}

	vkAPI := api.NewVK(cfg.VKToken)

	charService := application.NewCharacterService(charRepo)
	questService := application.NewQuestService(questRepo)
	sceneService := application.NewSceneService(sceneRepo)
	locService := application.NewLocationService(locRepo)

	gmService := application.NewGMService(cfg, sceneService, charService, llmClient, vkAPI, db)

	handler := vk.NewHandler(
		cfg,
		vkAPI,
		llmClient,
		charService,
		questService,
		sceneService,
		locService,
		gmService,
	)

	group := cfg.VKGroupID
	lp, err := longpoll.NewLongPoll(vkAPI, group)
	if err != nil {
		log.Fatalf("longpoll init error: %v", err)
	}

	handler.Start(lp)

	log.Println("Aurora bot started...")

	go func() {
		if err := lp.Run(); err != nil {
			log.Fatalf("longpoll error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
}
