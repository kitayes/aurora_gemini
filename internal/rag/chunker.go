package rag

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultChunkSize = 1000
	DefaultOverlap   = 200
	MaxChunkSize     = 2000
	MinChunkSize     = 100
	SummaryChunkSize = 300
)

type ChunkingStrategy string

const (
	StrategyFixed        ChunkingStrategy = "fixed"
	StrategyParagraph    ChunkingStrategy = "paragraph"
	StrategySentence     ChunkingStrategy = "sentence"
	StrategyHierarchical ChunkingStrategy = "hierarchical"
)

type DocumentChunk struct {
	ID       string
	Content  string
	Index    int
	Type     string
	Parent   string
	Metadata map[string]interface{}
}

type ChunkerOptions struct {
	Strategy  ChunkingStrategy
	ChunkSize int
	Overlap   int
	MinSize   int
}

func NewDefaultChunkerOptions() ChunkerOptions {
	return ChunkerOptions{
		Strategy:  StrategyParagraph,
		ChunkSize: DefaultChunkSize,
		Overlap:   DefaultOverlap,
		MinSize:   MinChunkSize,
	}
}

func ChunkDocument(docID, title, content string, opts ChunkerOptions) []DocumentChunk {
	switch opts.Strategy {
	case StrategyHierarchical:
		return chunkHierarchical(docID, title, content, opts)
	case StrategyParagraph:
		return chunkByParagraph(docID, content, opts)
	case StrategySentence:
		return chunkBySentence(docID, content, opts)
	default:
		return chunkFixed(docID, content, opts)
	}
}

func chunkHierarchical(docID, title, content string, opts ChunkerOptions) []DocumentChunk {
	var chunks []DocumentChunk

	summary := extractSummary(title, content, SummaryChunkSize)
	chunks = append(chunks, DocumentChunk{
		ID:      fmt.Sprintf("%s_summary", docID),
		Content: summary,
		Index:   0,
		Type:    "summary",
		Parent:  docID,
		Metadata: map[string]interface{}{
			"is_summary": true,
			"title":      title,
		},
	})

	sections := splitIntoSections(content)
	for i, section := range sections {
		if utf8.RuneCountInString(section) > opts.ChunkSize {
			subChunks := chunkByParagraph(
				fmt.Sprintf("%s_sec%d", docID, i),
				section,
				opts,
			)
			chunks = append(chunks, subChunks...)
		} else {
			chunks = append(chunks, DocumentChunk{
				ID:      fmt.Sprintf("%s_sec%d", docID, i),
				Content: section,
				Index:   i + 1,
				Type:    "section",
				Parent:  docID,
				Metadata: map[string]interface{}{
					"section_index": i,
				},
			})
		}
	}

	return chunks
}

func chunkByParagraph(docID, content string, opts ChunkerOptions) []DocumentChunk {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []DocumentChunk
	var currentChunk strings.Builder
	chunkIndex := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		if currentChunk.Len()+utf8.RuneCountInString(para) > opts.ChunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, DocumentChunk{
				ID:      fmt.Sprintf("%s_chunk%d", docID, chunkIndex),
				Content: strings.TrimSpace(currentChunk.String()),
				Index:   chunkIndex,
				Type:    "paragraph",
				Parent:  docID,
			})
			chunkIndex++

			if opts.Overlap > 0 {
				overlapText := getLastNChars(currentChunk.String(), opts.Overlap)
				currentChunk.Reset()
				currentChunk.WriteString(overlapText)
				currentChunk.WriteString("\n\n")
			} else {
				currentChunk.Reset()
			}
		}

		currentChunk.WriteString(para)
		currentChunk.WriteString("\n\n")
	}

	if currentChunk.Len() > opts.MinSize {
		chunks = append(chunks, DocumentChunk{
			ID:      fmt.Sprintf("%s_chunk%d", docID, chunkIndex),
			Content: strings.TrimSpace(currentChunk.String()),
			Index:   chunkIndex,
			Type:    "paragraph",
			Parent:  docID,
		})
	}

	return chunks
}

func chunkBySentence(docID, content string, opts ChunkerOptions) []DocumentChunk {
	sentences := splitSentences(content)
	var chunks []DocumentChunk
	var currentChunk []string
	currentSize := 0
	chunkIndex := 0

	for _, sentence := range sentences {
		sentenceSize := utf8.RuneCountInString(sentence)

		if currentSize+sentenceSize > opts.ChunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, DocumentChunk{
				ID:      fmt.Sprintf("%s_chunk%d", docID, chunkIndex),
				Content: strings.Join(currentChunk, " "),
				Index:   chunkIndex,
				Type:    "sentence",
				Parent:  docID,
			})
			chunkIndex++

			if opts.Overlap > 0 && len(currentChunk) > 1 {
				overlapSentences := currentChunk[len(currentChunk)-1:]
				currentChunk = overlapSentences
				currentSize = utf8.RuneCountInString(strings.Join(currentChunk, " "))
			} else {
				currentChunk = []string{}
				currentSize = 0
			}
		}

		currentChunk = append(currentChunk, sentence)
		currentSize += sentenceSize
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, DocumentChunk{
			ID:      fmt.Sprintf("%s_chunk%d", docID, chunkIndex),
			Content: strings.Join(currentChunk, " "),
			Index:   chunkIndex,
			Type:    "sentence",
			Parent:  docID,
		})
	}

	return chunks
}

func chunkFixed(docID, content string, opts ChunkerOptions) []DocumentChunk {
	runes := []rune(content)
	var chunks []DocumentChunk
	chunkIndex := 0

	for i := 0; i < len(runes); i += opts.ChunkSize - opts.Overlap {
		end := i + opts.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunk := string(runes[i:end])
		if utf8.RuneCountInString(chunk) >= opts.MinSize {
			chunks = append(chunks, DocumentChunk{
				ID:      fmt.Sprintf("%s_chunk%d", docID, chunkIndex),
				Content: chunk,
				Index:   chunkIndex,
				Type:    "fixed",
				Parent:  docID,
			})
			chunkIndex++
		}

		if end >= len(runes) {
			break
		}
	}

	return chunks
}

func extractSummary(title, content string, maxLength int) string {
	runes := []rune(content)
	if len(runes) <= maxLength {
		return content
	}

	summary := string(runes[:maxLength])

	lastPeriod := strings.LastIndex(summary, ".")
	if lastPeriod > maxLength/2 {
		summary = summary[:lastPeriod+1]
	}

	return fmt.Sprintf("%s\n\n%s...", title, summary)
}

func splitIntoSections(content string) []string {
	markers := []string{
		"\n## ", "\n### ", "\n#### ",
		"\n---\n", "\n***\n",
	}

	for _, marker := range markers {
		if strings.Contains(content, marker) {
			parts := strings.Split(content, marker)
			var sections []string
			for i, part := range parts {
				if i > 0 {
					part = marker + part
				}
				if strings.TrimSpace(part) != "" {
					sections = append(sections, strings.TrimSpace(part))
				}
			}
			if len(sections) > 1 {
				return sections
			}
		}
	}

	paragraphs := strings.Split(content, "\n\n")
	var sections []string
	var currentSection strings.Builder
	sectionSize := 1500

	for _, para := range paragraphs {
		if currentSection.Len()+len(para) > sectionSize && currentSection.Len() > 0 {
			sections = append(sections, currentSection.String())
			currentSection.Reset()
		}
		currentSection.WriteString(para)
		currentSection.WriteString("\n\n")
	}

	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	return sections
}

func splitSentences(text string) []string {
	text = strings.ReplaceAll(text, "\n", " ")

	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			if i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\n') {
				sentence := strings.TrimSpace(current.String())
				if len(sentence) > 0 {
					sentences = append(sentences, sentence)
				}
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if len(sentence) > 0 {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

func getLastNChars(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[len(runes)-n:])
}
