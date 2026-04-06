//go:build js && wasm

package main

// ClientConfig holds client-side configuration
type ClientConfig struct {
	DefaultRepo   string `json:"default_repo"`
	KanbanPath    string `json:"kanban_path"`
	ArchivePath   string `json:"archive_path"`
	AppInstallURL string `json:"app_install_url"`
}

// Session holds OAuth session data
type Session struct {
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
	ExpiresAt             string `json:"expires_at,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	RefreshTokenExpiresAt string `json:"refresh_token_expires_at,omitempty"`
	SelectedRepo          string `json:"selected_repo,omitempty"`
}

// User represents a GitHub user
type User struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// RepoSelection represents the currently selected repository
type RepoSelection struct {
	Owner       string
	Name        string
	Repo        string
	KanbanPath  string
	ArchivePath string
	Branch      string
}

// RepoOption represents a repository option in the dropdown
type RepoOption struct {
	Owner    string
	Name     string
	FullName string
}

// BoardConfig holds the kanban board configuration
type BoardConfig struct {
	Columns    []Column
	Categories []string
	Users      []string
	Tags       []string
}

// Column represents a kanban column
type Column struct {
	Name string
	ID   string
}

// Task represents a kanban task
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

// GetID returns the task ID
func (t Task) GetID() string {
	return t.ID
}

// Subtask represents a subtask within a task
type Subtask struct {
	ID        string
	Completed bool
	Text      string
	DueDate   string
}

// Filter represents a search/filter criterion
type Filter struct {
	Type  string
	Value string
}

// retryAction indicates what operation to retry after token refresh
type retryAction string

const (
	retryNone       retryAction = ""
	retryFetchRepos retryAction = "fetch_repos"
	retryLoadRepo   retryAction = "load_repo"
	retryCommit     retryAction = "commit"
)
