//go:build js && wasm

package main

// SetSession initializes the session from the server
type SetSession struct {
	Session string
}

// ViewerLoaded is returned after fetching the viewer
type ViewerLoaded struct {
	Viewer User
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

// LoadRepo triggers a manual repo reload
type LoadRepo struct{}

// CommitChanges triggers the commit modal
type CommitChanges struct{}

// CommitResult is returned after a commit attempt
type CommitResult struct {
	URL          string
	OID          string
	Error        string
	Unauthorized bool
	StaleHead    bool
}

// HeadRefreshed is returned after refreshing the head ref
type HeadRefreshed struct {
	HeadOID        string
	KanbanContent  string
	ArchiveContent string
	Error          string
}

// ReposLoaded is returned after fetching the list of repos
type ReposLoaded struct {
	Repos        []RepoOption
	Error        string
	Unauthorized bool
}

// SelectRepo changes the selected repository
type SelectRepo struct {
	FullName string
}

// OpenInstallURL opens the GitHub App install URL
type OpenInstallURL struct{}

// RepoSelectionSaved is returned after saving repo selection
type RepoSelectionSaved struct {
	Error string
}

// UpdateSearch updates the search query
type UpdateSearch struct {
	Value string
}

// AddFilter adds a filter
type AddFilter struct {
	Type  string
	Value string
}

// RemoveFilter removes a filter
type RemoveFilter struct {
	Index int
}

// ClearFilters clears all filters
type ClearFilters struct{}

// OpenModal opens a modal
type OpenModal struct {
	Mode   ModalMode
	TaskID string
}

// CloseModal closes a modal
type CloseModal struct {
	Mode ModalMode
}

// UpdateFormField updates a form field
type UpdateFormField struct {
	Field string
	Value string
}

// UpdateCommitMessage updates the commit message
type UpdateCommitMessage struct {
	Value string
}

// ClearStatus clears the status message
type ClearStatus struct {
	Seq int
}

// SetTagSuggestionsOpen controls tag suggestions visibility
type SetTagSuggestionsOpen struct {
	Open bool
}

// SelectTagSuggestion selects a tag suggestion
type SelectTagSuggestion struct {
	Tag string
}

// AddFormSubtask adds a subtask to the form
type AddFormSubtask struct {
	Text    string
	DueDate string
}

// ToggleFormSubtask toggles a form subtask completion
type ToggleFormSubtask struct {
	Index int
}

// UpdateFormSubtaskText updates a form subtask's text
type UpdateFormSubtaskText struct {
	Index int
	Value string
}

// UpdateFormSubtaskDueDate updates a form subtask's due date
type UpdateFormSubtaskDueDate struct {
	Index int
	Value string
}

// DeleteFormSubtask deletes a form subtask
type DeleteFormSubtask struct {
	Index int
}

// ConfirmCommit confirms and executes the commit
type ConfirmCommit struct {
	Message string
}

// UpdateDetailSubtaskField updates a detail subtask field
type UpdateDetailSubtaskField struct {
	Field string
	Value string
}

// AddTaskSubtask adds a subtask to a task
type AddTaskSubtask struct {
	TaskID  string
	Text    string
	DueDate string
}

// ToggleTaskSubtask toggles a task subtask completion
type ToggleTaskSubtask struct {
	TaskID string
	Index  int
}

// UpdateTaskSubtaskText updates a task subtask's text
type UpdateTaskSubtaskText struct {
	TaskID string
	Index  int
	Value  string
}

// UpdateTaskSubtaskDueDate updates a task subtask's due date
type UpdateTaskSubtaskDueDate struct {
	TaskID string
	Index  int
	Value  string
}

// DeleteTaskSubtask deletes a task subtask
type DeleteTaskSubtask struct {
	TaskID string
	Index  int
}

// SaveTask saves the current task
type SaveTask struct{}

// DeleteTask deletes a task
type DeleteTask struct {
	TaskID string
}

// ArchiveTask archives a task
type ArchiveTask struct {
	TaskID string
}

// RestoreTask restores an archived task
type RestoreTask struct {
	TaskID string
}

// MoveTaskPosition moves a task up or down
type MoveTaskPosition struct {
	TaskID    string
	Direction int
}

// CloneTask clones a task
type CloneTask struct {
	TaskID string
}

// AddColumn adds a new column
type AddColumn struct{}

// UpdateColumn updates a column
type UpdateColumn struct {
	Index int
	Field string
	Value string
}

// DeleteColumn deletes a column
type DeleteColumn struct {
	Index int
}

// MoveColumn moves a column
type MoveColumn struct {
	Index     int
	Direction int
}

// UpdateArchiveSearch updates the archive search
type UpdateArchiveSearch struct {
	Value string
}

// DragStartTask starts dragging a task
type DragStartTask struct {
	TaskID string
}

// DragOverTask fires when dragging over a task
type DragOverTask struct {
	TaskID   string
	ColumnID string
}

// DragOverColumn fires when dragging over a column
type DragOverColumn struct {
	ColumnID string
}

// DragEndTask ends the drag operation
type DragEndTask struct{}

// DropOnTask drops on a task
type DropOnTask struct {
	TargetTaskID string
	ColumnID     string
}

// DropOnColumn drops on a column
type DropOnColumn struct {
	ColumnID string
}

// OpenDetailFromTodo opens task detail from TODO view
type OpenDetailFromTodo struct {
	TaskID string
}

// Logout logs the user out
type Logout struct{}

// RefreshSession triggers a session refresh
type RefreshSession struct{}

// SessionRefreshed is returned after a token refresh attempt
type SessionRefreshed struct {
	Session Session
	Error   string
}

// SSEReload is sent when SSE indicates a reload is needed
type SSEReload struct {
	Repo    string
	HeadOID string
}

// SSEError is sent when SSE encounters an error
type SSEError struct {
	Repo       string
	RetryDelay int // milliseconds
}

// ModalMode represents the type of modal
type ModalMode string

const (
	ModalNone    ModalMode = "none"
	ModalDetail  ModalMode = "detail"
	ModalEdit    ModalMode = "edit"
	ModalArchive ModalMode = "archive"
	ModalColumns ModalMode = "columns"
	ModalCommit  ModalMode = "commit"
	ModalTodo    ModalMode = "todo"
)
