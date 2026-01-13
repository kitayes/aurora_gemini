package rag

import (
	"context"
	"fmt"
	"strings"

	"aurora/internal/embeddings"
	"aurora/internal/lore"
	"aurora/internal/repository"
)

type Service struct {
	embedder   embeddings.Service
	vectorRepo repository.VectorRepository
	loreRepo   lore.Repository
}

type RetrievalOptions struct {
	Limit    int
	Filters  map[string]string
	MinScore float32
}

func NewService(
	embedder embeddings.Service,
	vectorRepo repository.VectorRepository,
	loreRepo lore.Repository,
) *Service {
	return &Service{
		embedder:   embedder,
		vectorRepo: vectorRepo,
		loreRepo:   loreRepo,
	}
}

func (s *Service) RetrieveRelevant(ctx context.Context, query string, options RetrievalOptions) ([]lore.Chunk, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	if options.Limit <= 0 {
		options.Limit = 5
	}

	queryVector, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return s.fallbackToTagSearch(ctx, options)
	}

	docs, err := s.vectorRepo.SearchSimilar(ctx, queryVector, options.Limit, options.Filters)
	if err != nil {
		return s.fallbackToTagSearch(ctx, options)
	}

	if len(docs) == 0 {
		return s.fallbackToTagSearch(ctx, options)
	}

	chunks := make([]lore.Chunk, 0, len(docs))
	for _, doc := range docs {
		chunks = append(chunks, lore.Chunk{
			Title:   doc.Title,
			Content: doc.Content,
			Zone:    doc.Zone,
			Tags:    doc.Tags,
		})
	}

	return chunks, nil
}

func (s *Service) fallbackToTagSearch(ctx context.Context, options RetrievalOptions) ([]lore.Chunk, error) {
	locationTag := ""
	factionTag := ""
	extraTags := []string{}

	if zone, ok := options.Filters["zone"]; ok {
		locationTag = zone
	}
	if faction, ok := options.Filters["faction"]; ok {
		factionTag = faction
	}
	if tags, ok := options.Filters["tags"]; ok {
		extraTags = strings.Split(tags, ",")
	}

	return s.loreRepo.SelectRelevant(locationTag, factionTag, extraTags), nil
}

func (s *Service) IndexLore(ctx context.Context) error {
	coreLore := s.loreRepo.GetCoreLore()
	if coreLore != "" {
		coreDoc := repository.VectorDocument{
			ID:      "core",
			Title:   "Базовый лор мира Аврора",
			Content: coreLore,
			Zone:    "world",
			Tags:    []string{"world", "core", "основа"},
			Metadata: map[string]string{
				"type": "core",
			},
		}

		vector, err := s.embedder.Embed(ctx, coreLore)
		if err != nil {
			return fmt.Errorf("embed core lore: %w", err)
		}
		coreDoc.Vector = vector

		if err := s.vectorRepo.IndexDocument(ctx, coreDoc); err != nil {
			return fmt.Errorf("index core lore: %w", err)
		}
	}

	chunks := s.loreRepo.SelectRelevant("", "", []string{})

	var docs []repository.VectorDocument
	for i, chunk := range chunks {
		docID := fmt.Sprintf("chunk_%d_%s", i, chunk.Zone)

		combinedText := fmt.Sprintf("%s\n\n%s", chunk.Title, chunk.Content)
		vector, err := s.embedder.Embed(ctx, combinedText)
		if err != nil {
			continue
		}

		doc := repository.VectorDocument{
			ID:      docID,
			Title:   chunk.Title,
			Content: chunk.Content,
			Zone:    chunk.Zone,
			Tags:    chunk.Tags,
			Vector:  vector,
			Metadata: map[string]string{
				"source": "lore_file",
			},
		}

		docs = append(docs, doc)
	}

	if len(docs) > 0 {
		if err := s.vectorRepo.IndexBatch(ctx, docs); err != nil {
			return fmt.Errorf("index chunks: %w", err)
		}
	}

	return nil
}

func (s *Service) GetStats(ctx context.Context) (repository.VectorStats, error) {
	return s.vectorRepo.GetStats(ctx)
}

func (s *Service) ReindexAll(ctx context.Context) error {
	if err := s.vectorRepo.DeleteAll(ctx); err != nil {
		return fmt.Errorf("delete old vectors: %w", err)
	}

	return s.IndexLore(ctx)
}

func (s *Service) IndexLargeDocument(ctx context.Context, docID, title, content, zone string, tags []string, strategy ChunkingStrategy) error {
	opts := NewDefaultChunkerOptions()
	opts.Strategy = strategy

	if strategy == StrategyHierarchical {
		return s.indexHierarchical(ctx, docID, title, content, zone, tags, opts)
	}

	docChunks := ChunkDocument(docID, title, content, opts)

	var vectorDocs []repository.VectorDocument
	for _, chunk := range docChunks {
		vector, err := s.embedder.Embed(ctx, chunk.Content)
		if err != nil {
			continue
		}

		vectorDocs = append(vectorDocs, repository.VectorDocument{
			ID:      chunk.ID,
			Title:   fmt.Sprintf("%s (часть %d)", title, chunk.Index+1),
			Content: chunk.Content,
			Zone:    zone,
			Tags:    tags,
			Vector:  vector,
			Metadata: map[string]string{
				"parent":      chunk.Parent,
				"chunk_type":  chunk.Type,
				"chunk_index": fmt.Sprintf("%d", chunk.Index),
			},
		})
	}

	if len(vectorDocs) > 0 {
		return s.vectorRepo.IndexBatch(ctx, vectorDocs)
	}

	return nil
}

func (s *Service) indexHierarchical(ctx context.Context, docID, title, content, zone string, tags []string, opts ChunkerOptions) error {
	docChunks := ChunkDocument(docID, title, content, opts)

	var vectorDocs []repository.VectorDocument
	for _, chunk := range docChunks {
		vector, err := s.embedder.Embed(ctx, chunk.Content)
		if err != nil {
			continue
		}

		chunkTitle := title
		if chunk.Type == "summary" {
			chunkTitle = fmt.Sprintf("%s (краткое содержание)", title)
		} else if chunk.Type == "section" {
			chunkTitle = fmt.Sprintf("%s (раздел %d)", title, chunk.Index)
		}

		metadata := map[string]string{
			"parent":      chunk.Parent,
			"chunk_type":  chunk.Type,
			"chunk_index": fmt.Sprintf("%d", chunk.Index),
		}

		if chunk.Type == "summary" {
			metadata["is_summary"] = "true"
		}

		vectorDocs = append(vectorDocs, repository.VectorDocument{
			ID:       chunk.ID,
			Title:    chunkTitle,
			Content:  chunk.Content,
			Zone:     zone,
			Tags:     tags,
			Vector:   vector,
			Metadata: metadata,
		})
	}

	if len(vectorDocs) > 0 {
		return s.vectorRepo.IndexBatch(ctx, vectorDocs)
	}

	return nil
}

func (s *Service) IndexFromText(ctx context.Context, docID, title, content, zone string, tags []string) error {
	contentLength := len([]rune(content))

	var strategy ChunkingStrategy
	if contentLength > 5000 {
		strategy = StrategyHierarchical
	} else if contentLength > 2000 {
		strategy = StrategyParagraph
	} else {
		vector, err := s.embedder.Embed(ctx, fmt.Sprintf("%s\n\n%s", title, content))
		if err != nil {
			return err
		}

		doc := repository.VectorDocument{
			ID:      docID,
			Title:   title,
			Content: content,
			Zone:    zone,
			Tags:    tags,
			Vector:  vector,
			Metadata: map[string]string{
				"source": "direct",
			},
		}

		return s.vectorRepo.IndexDocument(ctx, doc)
	}

	return s.IndexLargeDocument(ctx, docID, title, content, zone, tags, strategy)
}
