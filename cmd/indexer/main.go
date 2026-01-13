package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"aurora/internal/embeddings"
	"aurora/internal/lore"
	"aurora/internal/rag"
	"aurora/internal/repository"
	"aurora/pkg/config"
)

func main() {
	log.Println("🔍 Aurora Lore Indexer")
	log.Println("====================")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	db, err := repository.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database")

	loreRepo, err := lore.NewFileLoreRepo("lore")
	if err != nil {
		log.Fatalf("❌ Failed to load lore: %v", err)
	}
	log.Println("✅ Loaded lore repository")

	embedder := embeddings.NewGeminiEmbedder(cfg.GeminiKey)
	log.Println("✅ Initialized embedding service")

	vectorRepo := repository.NewVectorRepository(db)
	log.Println("✅ Initialized vector repository")

	ragService := rag.NewService(embedder, vectorRepo, loreRepo)
	log.Println("✅ Initialized RAG service")

	ctx := context.Background()

	if len(os.Args) > 1 && os.Args[1] == "--reindex" {
		log.Println("🔄 Reindexing all lore (deleting old vectors)...")
		if err := ragService.ReindexAll(ctx); err != nil {
			log.Fatalf("❌ Reindex failed: %v", err)
		}
	} else {
		log.Println("📚 Indexing lore...")
		if err := ragService.IndexLore(ctx); err != nil {
			log.Fatalf("❌ Indexing failed: %v", err)
		}
	}

	stats, err := ragService.GetStats(ctx)
	if err != nil {
		log.Printf("⚠️  Failed to get stats: %v", err)
	} else {
		log.Printf("✅ Indexing complete!")
		log.Printf("   Total documents: %d", stats.TotalDocuments)
		log.Println("   Documents by zone:")
		for zone, count := range stats.DocumentsByZone {
			log.Printf("     - %s: %d", zone, count)
		}
	}

	log.Println("\n🎉 Done! You can now use semantic search in your bot.")

	if len(os.Args) <= 1 {
		fmt.Println("\nTip: Use --reindex flag to delete and reindex all vectors")
	}
}
