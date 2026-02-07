//go:build js && wasm

package main

type ClientConfig struct {
	DefaultRepo   string `json:"default_repo"`
	KanbanPath    string `json:"kanban_path"`
	ArchivePath   string `json:"archive_path"`
	CommitMessage string `json:"commit_message"`
}

type Session struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

type User struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type RepoSelection struct {
	Owner       string
	Name        string
	Repo        string
	KanbanPath  string
	ArchivePath string
	Branch      string
}

type BoardConfig struct {
	Columns    []Column
	Categories []string
	Users      []string
	Tags       []string
}

type Column struct {
	Name string
	ID   string
}

type Task struct {
	ID          string
	Title       string
	Status      string
	Category    string
	Assignees   []string
	Tags        []string
	Created     string
	Modified    string
	Completed   string
	Description string
	Subtasks    []Subtask
	Notes       string
}

type Subtask struct {
	Completed bool
	Text      string
	DueDate   string
}

type Filter struct {
	Type  string
	Value string
}
