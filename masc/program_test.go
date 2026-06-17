//go:build !js

package main

import (
	"testing"
)

// testProgram is a minimal version of Program for testing the retry logic
type testProgram struct {
	loggedIn     bool
	loading      bool
	token        string
	session      Session
	pendingRetry retryAction
	repos        []RepoOption
	selectedRepo string
	repoLoaded   bool
	error        string
}

// handleUnauthorized mirrors the logic from Program.handleUnauthorized
func (p *testProgram) handleUnauthorized(retry retryAction) bool {
	if p.session.RefreshToken != "" {
		p.pendingRetry = retry
		return true // would return refreshSessionCmd
	}
	// Clear state when no refresh token
	p.loggedIn = false
	p.token = ""
	p.session = Session{}
	p.loading = false
	p.pendingRetry = retryNone
	p.repos = nil
	p.repoLoaded = false
	p.selectedRepo = ""
	return false
}

// handleViewerLoadError mirrors the logic from Program.Update for ViewerLoadError
func (p *testProgram) handleViewerLoadError(msg ViewerLoadError) bool {
	if msg.Unauthorized {
		return p.handleUnauthorized(retryNone) // KEY: uses retryNone, not retryLoadRepo
	}
	p.loading = false
	p.error = msg.Error
	return false
}

// handleLoadError mirrors the logic from Program.Update for LoadError
func (p *testProgram) handleLoadError(msg LoadError) bool {
	if msg.Unauthorized {
		return p.handleUnauthorized(retryLoadRepo)
	}
	p.loading = false
	p.error = msg.Error
	return false
}

// handleSessionRefreshed mirrors the logic from Program.handleSessionRefreshed
func (p *testProgram) handleSessionRefreshed(msg SessionRefreshed) string {
	if msg.Error != "" {
		p.loggedIn = false
		p.token = ""
		p.session = Session{}
		p.loading = false
		p.pendingRetry = retryNone
		p.repos = nil
		p.repoLoaded = false
		p.selectedRepo = ""
		return "clearSession"
	}

	p.session = msg.Session
	p.token = msg.Session.AccessToken
	p.error = ""

	retry := p.pendingRetry
	p.pendingRetry = retryNone

	switch retry {
	case retryFetchRepos:
		p.loading = true
		return "fetchRepos"
	case retryLoadRepo:
		p.loading = true
		return "loadRepo"
	case retryCommit:
		return "commit"
	default:
		p.loading = true
		return "fetchViewer" // KEY: default case goes back to fetchViewer
	}
}

// Tests for the ViewerLoadError fix

func TestViewerLoadError_Unauthorized_Uses_RetryNone(t *testing.T) {
	p := &testProgram{
		loggedIn: true,
		token:    "test-token",
		session:  Session{RefreshToken: "refresh-token"},
	}

	hasCmd := p.handleViewerLoadError(ViewerLoadError{Error: "unauthorized", Unauthorized: true})

	if !hasCmd {
		t.Error("expected a command to be returned for token refresh")
	}

	// Should have set pendingRetry to retryNone for proper re-fetch flow
	if p.pendingRetry != retryNone {
		t.Errorf("expected pendingRetry to be retryNone, got %q", p.pendingRetry)
	}
}

func TestViewerLoadError_NotUnauthorized_SetsError(t *testing.T) {
	p := &testProgram{
		loggedIn: true,
		loading:  true,
	}

	hasCmd := p.handleViewerLoadError(ViewerLoadError{Error: "network error", Unauthorized: false})

	if hasCmd {
		t.Error("expected no command for non-unauthorized error")
	}

	if p.error != "network error" {
		t.Errorf("expected error to be 'network error', got %q", p.error)
	}

	if p.loading {
		t.Error("expected loading to be false")
	}
}

func TestLoadError_Unauthorized_Uses_RetryLoadRepo(t *testing.T) {
	p := &testProgram{
		loggedIn: true,
		token:    "test-token",
		session:  Session{RefreshToken: "refresh-token"},
		repos:    []RepoOption{{FullName: "owner/repo"}},
	}

	hasCmd := p.handleLoadError(LoadError{Error: "unauthorized", Unauthorized: true})

	if !hasCmd {
		t.Error("expected a command to be returned for token refresh")
	}

	// Should have set pendingRetry to retryLoadRepo
	if p.pendingRetry != retryLoadRepo {
		t.Errorf("expected pendingRetry to be retryLoadRepo, got %q", p.pendingRetry)
	}
}

func TestHandleSessionRefreshed_RetryNone_CallsFetchViewer(t *testing.T) {
	p := &testProgram{
		loggedIn:     true,
		pendingRetry: retryNone,
		session:      Session{AccessToken: "old-token"},
	}

	cmd := p.handleSessionRefreshed(SessionRefreshed{
		Session: Session{AccessToken: "new-token"},
	})

	if p.token != "new-token" {
		t.Errorf("expected token to be 'new-token', got %q", p.token)
	}

	// Should return fetchViewer (the default case)
	if cmd != "fetchViewer" {
		t.Errorf("expected fetchViewer command, got %q", cmd)
	}

	if !p.loading {
		t.Error("expected loading to be true")
	}
}

func TestHandleSessionRefreshed_RetryLoadRepo_CallsLoadRepo(t *testing.T) {
	p := &testProgram{
		loggedIn:     true,
		pendingRetry: retryLoadRepo,
		session:      Session{AccessToken: "old-token"},
		selectedRepo: "owner/repo",
	}

	cmd := p.handleSessionRefreshed(SessionRefreshed{
		Session: Session{AccessToken: "new-token"},
	})

	if p.token != "new-token" {
		t.Errorf("expected token to be 'new-token', got %q", p.token)
	}

	// Should return loadRepo
	if cmd != "loadRepo" {
		t.Errorf("expected loadRepo command, got %q", cmd)
	}

	if !p.loading {
		t.Error("expected loading to be true")
	}

	// pendingRetry should be cleared
	if p.pendingRetry != retryNone {
		t.Errorf("expected pendingRetry to be cleared, got %q", p.pendingRetry)
	}
}

func TestHandleSessionRefreshed_RetryFetchRepos_CallsFetchRepos(t *testing.T) {
	p := &testProgram{
		loggedIn:     true,
		pendingRetry: retryFetchRepos,
		session:      Session{AccessToken: "old-token"},
	}

	cmd := p.handleSessionRefreshed(SessionRefreshed{
		Session: Session{AccessToken: "new-token"},
	})

	if cmd != "fetchRepos" {
		t.Errorf("expected fetchRepos command, got %q", cmd)
	}
}

// TestViewerLoadError_Flow_PreservesRepos tests the complete flow when
// fetchViewerCmd fails with unauthorized and token is refreshed.
// This is the bug scenario that was fixed.
func TestViewerLoadError_Flow_PreservesRepos(t *testing.T) {
	// Initial state: user was previously logged in, token has expired
	// repos is NOT set because this is a fresh page load
	p := &testProgram{
		loggedIn: true,
		token:    "expired-token",
		session:  Session{RefreshToken: "refresh-token"},
	}

	// Step 1: fetchViewerCmd fails with unauthorized
	p.handleViewerLoadError(ViewerLoadError{Error: "unauthorized", Unauthorized: true})

	// Verify retryNone is set (not retryLoadRepo!)
	if p.pendingRetry != retryNone {
		t.Fatalf("after ViewerLoadError, expected pendingRetry=retryNone, got %q", p.pendingRetry)
	}

	// Step 2: Token refresh succeeds
	cmd := p.handleSessionRefreshed(SessionRefreshed{Session: Session{AccessToken: "new-token"}})

	// Verify: the default case triggers fetchViewer, NOT loadRepo
	// This means repos will be fetched before loading the repo.
	if cmd != "fetchViewer" {
		t.Errorf("expected fetchViewer command after refresh with retryNone, got %q", cmd)
	}

	// repoLoaded should still be false (we haven't loaded a repo yet)
	if p.repoLoaded {
		t.Error("repoLoaded should be false - we need to go through the full flow")
	}

	// repos should still be empty/nil (we need to fetch them first)
	if len(p.repos) > 0 {
		t.Error("repos should be empty - we need to fetch them first")
	}
}

// TestLoadError_Flow_SkipsReposFetch tests that when loadRepoCmd fails,
// it correctly retries just the load (not the full flow), because repos
// were already fetched.
func TestLoadError_Flow_SkipsReposFetch(t *testing.T) {
	// State: repos were fetched, now loading the repo content
	p := &testProgram{
		loggedIn: true,
		token:    "expired-token",
		session:  Session{RefreshToken: "refresh-token"},
		repos:    []RepoOption{{FullName: "owner/repo"}},
	}

	// Step 1: loadRepoCmd fails with unauthorized
	p.handleLoadError(LoadError{Error: "unauthorized", Unauthorized: true})

	// Verify retryLoadRepo is set
	if p.pendingRetry != retryLoadRepo {
		t.Fatalf("after LoadError, expected pendingRetry=retryLoadRepo, got %q", p.pendingRetry)
	}

	// Step 2: Token refresh succeeds
	cmd := p.handleSessionRefreshed(SessionRefreshed{Session: Session{AccessToken: "new-token"}})

	// Should retry just loadRepo, not fetchViewer
	if cmd != "loadRepo" {
		t.Errorf("expected loadRepo command after refresh with retryLoadRepo, got %q", cmd)
	}

	// repos should still be set (they were fetched before the error)
	if len(p.repos) != 1 {
		t.Errorf("repos should still have 1 item, got %d", len(p.repos))
	}
}

// todoSubtask mirrors the subset of Subtask fields used by the TODO modal.
type todoSubtask struct {
	ID        string
	Text      string
	Completed bool
}

// todoTask mirrors the subset of Task fields used by the TODO modal.
type todoTask struct {
	ID       string
	Subtasks []todoSubtask
}

// testTodoItem mirrors todoItem for the !js test build.
type testTodoItem struct {
	TaskID       string
	SubtaskIndex int
	SubtaskID    string
	SubtaskText  string
}

// buildTestTodoItems mirrors Program.buildTodoItems: it includes only
// incomplete subtasks and carries each subtask's stable ID (used as the
// render key so checkbox DOM state doesn't bleed onto the next item).
func buildTestTodoItems(tasks []todoTask) []testTodoItem {
	items := make([]testTodoItem, 0)
	for _, task := range tasks {
		for idx, st := range task.Subtasks {
			if st.Completed {
				continue
			}
			items = append(items, testTodoItem{
				TaskID:       task.ID,
				SubtaskIndex: idx,
				SubtaskID:    st.ID,
				SubtaskText:  st.Text,
			})
		}
	}
	return items
}

// TestTodoItems_excludes_completed_subtasks verifies the TODO list drops
// completed subtasks so checking one removes it from the modal.
func TestTodoItems_excludes_completed_subtasks(t *testing.T) {
	tasks := []todoTask{{
		ID: "T1",
		Subtasks: []todoSubtask{
			{ID: "a", Text: "first", Completed: true},
			{ID: "b", Text: "second"},
			{ID: "c", Text: "third"},
		},
	}}

	items := buildTestTodoItems(tasks)

	if len(items) != 2 {
		t.Fatalf("expected 2 incomplete items, got %d", len(items))
	}
	if items[0].SubtaskText != "second" || items[1].SubtaskText != "third" {
		t.Errorf("unexpected items: %+v", items)
	}
}

// TestTodoItems_carry_stable_subtask_id verifies each rendered TODO row can
// be keyed by the subtask's own ID rather than its position. Without a stable
// key, removing the checked row reuses its checkbox DOM node for the next row,
// which then appears checked.
func TestTodoItems_carry_stable_subtask_id(t *testing.T) {
	tasks := []todoTask{{
		ID: "T1",
		Subtasks: []todoSubtask{
			{ID: "a", Text: "first"},
			{ID: "b", Text: "second"},
		},
	}}

	items := buildTestTodoItems(tasks)

	keys := map[string]bool{}
	for _, it := range items {
		if it.SubtaskID == "" {
			t.Fatalf("item %q is missing a subtask ID to key on", it.SubtaskText)
		}
		key := it.TaskID + "/" + it.SubtaskID
		if keys[key] {
			t.Fatalf("duplicate render key %q", key)
		}
		keys[key] = true
	}
}
