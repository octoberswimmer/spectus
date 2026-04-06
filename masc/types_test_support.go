//go:build !js

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
