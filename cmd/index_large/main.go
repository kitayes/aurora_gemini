package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"aurora/internal/embeddings"
	"aurora/internal/lore"
	"aurora/internal/rag"
	"aurora/internal/repository"
	"aurora/pkg/config"
)

func main() {
	dirPtr := flag.String("dir", "", "Directory with large text files")
	filePtr := flag.String("file", "", "Single file to index")
	zonePtr := flag.String("zone", "extended_lore", "Zone/category for documents")
	tagsPtr := flag.String("tags", "", "Comma-separated tags")
	strategyPtr := flag.String("strategy", "auto", "Chunking strategy: auto, hierarchical, paragraph, sentence")

	flag.Parse()

	if *dirPtr == "" && *filePtr == "" {
		log.Fatal("❌ Please specify either --dir or --file")
	}

	log.Println("📚 Large Document Indexer")
	log.Println("==========================")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	db, err := repository.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	loreRepo, err := lore.NewFileLoreRepo("lore")
	if err != nil {
		log.Printf("⚠️  Failed to load lore repo: %v", err)
	}

	embedder := embeddings.NewGeminiEmbedder(cfg.GeminiKey)
	vectorRepo := repository.NewVectorRepository(db)
	ragService := rag.NewService(embedder, vectorRepo, loreRepo)

	log.Println("✅ Initialized RAG service")

	ctx := context.Background()

	tags := parseTags(*tagsPtr)
	strategy := parseStrategy(*strategyPtr)

	if *filePtr != "" {
		if err := indexFile(ctx, ragService, *filePtr, *zonePtr, tags, strategy); err != nil {
			log.Fatalf("❌ Failed to index file: %v", err)
		}
	} else if *dirPtr != "" {
		if err := indexDirectory(ctx, ragService, *dirPtr, *zonePtr, tags, strategy); err != nil {
			log.Fatalf("❌ Failed to index directory: %v", err)
		}
	}

	stats, err := ragService.GetStats(ctx)
	if err != nil {
		log.Printf("⚠️  Failed to get stats: %v", err)
	} else {
		log.Println("\n📊 Final Statistics:")
		log.Printf("   Total documents: %d", stats.TotalDocuments)
		log.Println("   Documents by zone:")
		for zone, count := range stats.DocumentsByZone {
			log.Printf("     - %s: %d", zone, count)
		}
	}

	log.Println("\n🎉 Indexing complete!")
}

func indexFile(ctx context.Context, ragService *rag.Service, filePath, zone string, tags []string, strategy rag.ChunkingStrategy) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	filename := filepath.Base(filePath)
	docID := strings.TrimSuffix(filename, filepath.Ext(filename))
	title := formatTitle(docID)

	log.Printf("📄 Indexing: %s", filename)
	log.Printf("   Size: %d characters", len(content))
	log.Printf("   Strategy: %s", strategy)

	if strategy == "auto" {
		err = ragService.IndexFromText(ctx, docID, title, string(content), zone, tags)
	} else {
		err = ragService.IndexLargeDocument(ctx, docID, title, string(content), zone, tags, strategy)
	}

	if err != nil {
		return err
	}

	log.Println("   ✅ Indexed successfully")
	return nil
}

func indexDirectory(ctx context.Context, ragService *rag.Service, dirPath, zone string, tags []string, strategy rag.ChunkingStrategy) error {
	extensions := []string{".txt", ".md", ".markdown"}
	var files []string

	for _, ext := range extensions {
		pattern := filepath.Join(dirPath, "*"+ext)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in %s", dirPath)
	}

	log.Printf("📁 Found %d files in %s\n", len(files), dirPath)

	for i, file := range files {
		log.Printf("\n[%d/%d] ", i+1, len(files))
		if err := indexFile(ctx, ragService, file, zone, tags, strategy); err != nil {
			log.Printf("   ⚠️  Error: %v", err)
		}
	}

	return nil
}

func parseTags(tagsStr string) []string {
	if tagsStr == "" {
		return []string{}
	}

	parts := strings.Split(tagsStr, ",")
	var tags []string
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func parseStrategy(strategyStr string) rag.ChunkingStrategy {
	switch strings.ToLower(strategyStr) {
	case "hierarchical":
		return rag.StrategyHierarchical
	case "paragraph":
		return rag.StrategyParagraph
	case "sentence":
		return rag.StrategySentence
	case "fixed":
		return rag.StrategyFixed
	default:
		return "auto"
	}
}

func formatTitle(filename string) string {
	name := strings.ReplaceAll(filename, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}

	return name
}
