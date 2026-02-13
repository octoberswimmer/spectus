package pending

import (
	"encoding/json"
	"strings"
)

const StorageKeyPrefix = "spectus_pending_"

type Changes struct {
	Repo            string `json:"repo"`
	KanbanMarkdown  string `json:"kanban"`
	ArchiveMarkdown string `json:"archive"`
	CommitMessage   string `json:"commitMessage,omitempty"`
}

type Storage interface {
	GetItem(key string) (string, bool)
	SetItem(key, value string)
	RemoveItem(key string)
}

func storageKey(repo string) string {
	return StorageKeyPrefix + strings.ToLower(repo)
}

func Save(storage Storage, repo, kanbanMarkdown, archiveMarkdown, commitMessage string) error {
	data := Changes{
		Repo:            repo,
		KanbanMarkdown:  kanbanMarkdown,
		ArchiveMarkdown: archiveMarkdown,
		CommitMessage:   commitMessage,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	storage.SetItem(storageKey(repo), string(jsonData))
	return nil
}

func Load(storage Storage, repo string) *Changes {
	val, ok := storage.GetItem(storageKey(repo))
	if !ok || val == "" {
		return nil
	}
	var data Changes
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil
	}
	return &data
}

func Clear(storage Storage, repo string) {
	storage.RemoveItem(storageKey(repo))
}

func HasPending(saved *Changes) bool {
	return saved != nil
}
