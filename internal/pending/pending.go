package pending

import (
	"encoding/json"
	"strings"
)

const StorageKey = "spectus_pending_changes"

type Changes struct {
	Repo            string `json:"repo"`
	KanbanMarkdown  string `json:"kanban"`
	ArchiveMarkdown string `json:"archive"`
}

type Storage interface {
	GetItem(key string) (string, bool)
	SetItem(key, value string)
	RemoveItem(key string)
}

func Save(storage Storage, repo, kanbanMarkdown, archiveMarkdown string) error {
	data := Changes{
		Repo:            repo,
		KanbanMarkdown:  kanbanMarkdown,
		ArchiveMarkdown: archiveMarkdown,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	storage.SetItem(StorageKey, string(jsonData))
	return nil
}

func Load(storage Storage) *Changes {
	val, ok := storage.GetItem(StorageKey)
	if !ok || val == "" {
		return nil
	}
	var data Changes
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil
	}
	return &data
}

func Clear(storage Storage) {
	storage.RemoveItem(StorageKey)
}

func ShouldRestore(saved *Changes, loadedRepo string) bool {
	if saved == nil {
		return false
	}
	return strings.EqualFold(saved.Repo, loadedRepo)
}
