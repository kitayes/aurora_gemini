package lore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Chunk struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Zone    string   `json:"zone"` // world/region/city/faction/magic/economy
	Tags    []string `json:"tags"`
}

type Repository interface {
	GetCoreLore() string
	GetMasterInstruction() string
	SelectRelevant(locationTag, factionTag string, extraTags []string) []Chunk
}

type fileRepo struct {
	core              string
	masterInstruction string
	chunks            []Chunk
}

func NewFileLoreRepo(dir string) (Repository, error) {
	fr := &fileRepo{}
	if err := fr.load(dir); err != nil {
		return nil, err
	}
	return fr, nil
}

func (r *fileRepo) load(dir string) error {
	corePath := filepath.Join(dir, "core.json")
	data, err := os.ReadFile(corePath)
	if err != nil {
		return fmt.Errorf("read core.json: %w", err)
	}
	var c struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("parse core.json: %w", err)
	}
	r.core = c.Text

	masterPath := filepath.Join(dir, "master.json")
	if masterData, err := os.ReadFile(masterPath); err == nil {
		r.masterInstruction = string(masterData)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "core.json" || e.Name() == "master.json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var chunks []Chunk
		if err := json.Unmarshal(b, &chunks); err != nil {
			return fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		r.chunks = append(r.chunks, chunks...)
	}
	return nil
}

func (r *fileRepo) GetCoreLore() string {
	return r.core
}

func (r *fileRepo) SelectRelevant(locationTag, factionTag string, extraTags []string) []Chunk {
	var res []Chunk
	all := map[string]struct{}{}
	add := func(s string) {
		if s == "" {
			return
		}
		all[strings.ToLower(s)] = struct{}{}
	}
	add(locationTag)
	add(factionTag)
	for _, t := range extraTags {
		add(t)
	}

	for _, ch := range r.chunks {
		match := false
		for _, t := range ch.Tags {
			if _, ok := all[strings.ToLower(t)]; ok {
				match = true
				break
			}
		}
		if match {
			res = append(res, ch)
		}
		if len(res) >= 5 {
			break
		}
	}
	return res
}

func (r *fileRepo) GetMasterInstruction() string {
	if r.masterInstruction == "" {
		return ""
	}
	return r.masterInstruction
}
