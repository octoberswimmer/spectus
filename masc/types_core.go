package main

// retryAction indicates what operation to retry after token refresh
type retryAction string

const (
	retryNone       retryAction = ""
	retryFetchRepos retryAction = "fetch_repos"
	retryLoadRepo   retryAction = "load_repo"
	retryCommit     retryAction = "commit"
)

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

// RepoOption represents a repository option in the dropdown
type RepoOption struct {
	Owner    string
	Name     string
	FullName string
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

// ClientConfig holds client-side configuration
type ClientConfig struct {
	AppInstallURL string `json:"app_install_url"`
	DefaultRepo   string `json:"default_repo"`
	KanbanPath    string `json:"kanban_path"`
	ArchivePath   string `json:"archive_path"`
}

// User represents a GitHub user
type User struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Core message types for Update handling

// ViewerLoadError is returned when fetching the viewer fails
type ViewerLoadError struct {
	Error        string
	Unauthorized bool
}

// LoadError is returned when loading a repo fails
type LoadError struct {
	Error        string
	Unauthorized bool
}

// SessionRefreshed is returned after a token refresh attempt
type SessionRefreshed struct {
	Session Session
	Error   string
}

// ReposLoaded is returned after fetching the list of repos
type ReposLoaded struct {
	Repos        []RepoOption
	Error        string
	Unauthorized bool
}

// RepoLoaded is returned after loading a repo's content
type RepoLoaded struct {
	Repo           RepoSelection
	Branch         string
	HeadOID        string
	KanbanContent  string
	ArchiveContent string
	MissingKanban  bool
	MissingArchive bool
}

// ViewerLoaded is returned after fetching the viewer
type ViewerLoaded struct {
	Viewer User
}
