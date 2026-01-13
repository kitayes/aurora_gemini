package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"aurora/internal/delivery/vk"
	"aurora/internal/embeddings"
	"aurora/internal/llm"
	"aurora/internal/lore"
	"aurora/internal/rag"
	"aurora/internal/repository"
	"aurora/internal/service"
	"aurora/pkg/config"

	"github.com/SevereCloud/vksdk/v2/api"
	longpoll "github.com/SevereCloud/vksdk/v2/longpoll-bot"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := repository.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

	// Repositories
	charRepo := repository.NewCharacterRepository(db)
	questRepo := repository.NewQuestRepository(db)
	sceneRepo := repository.NewSceneRepository(db)
	locRepo := repository.NewLocationRepository(db)
	vectorRepo := repository.NewVectorRepository(db)

	// Lore
	loreRepo, err := lore.NewFileLoreRepo("lore")
	if err != nil {
		log.Printf("lore init failed: %v", err)
	}

	// LLM Client
	var llmClient llm.Client
	if cfg.LLMProvider == "openai" {
		log.Fatal("OpenAI not implemented")
	} else {
		llmClient = llm.NewGeminiClient(cfg.GeminiKey, cfg.LLMModel, loreRepo)
	}

	// RAG
	embedder := embeddings.NewGeminiEmbedder(cfg.GeminiKey)
	ragService := rag.NewService(embedder, vectorRepo, loreRepo)
	if gc, ok := llmClient.(*llm.GeminiClient); ok {
		gc.SetRAGService(ragService)
		log.Println("✅ RAG enabled")
	}

	// VK API
	vkAPI := api.NewVK(cfg.VKToken)

	// Services
	charService := service.NewCharacterService(charRepo)
	questService := service.NewQuestService(questRepo)
	sceneService := service.NewSceneService(sceneRepo)
	locService := service.NewLocationService(locRepo)
	gmService := service.NewGMService(cfg, sceneService, charService, llmClient, vkAPI, db)

	// Handler
	handler := vk.NewHandler(cfg, vkAPI, llmClient, charService, questService, sceneService, locService, gmService)

	// LongPoll
	lp, err := longpoll.NewLongPoll(vkAPI, cfg.VKGroupID)
	if err != nil {
		log.Fatalf("longpoll error: %v", err)
	}

	handler.Start(lp)
	log.Println("Aurora started...")

	go func() {
		if err := lp.Run(); err != nil {
			log.Fatalf("lp run error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown...")
}
