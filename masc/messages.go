//go:build js && wasm

package main

type SetSession struct {
	Session string
}

type ViewerLoaded struct {
	Viewer User
}

type LoadError struct {
	Error        string
	Unauthorized bool
}

type RepoLoaded struct {
	Repo           RepoSelection
	Branch         string
	HeadOID        string
	KanbanContent  string
	ArchiveContent string
	MissingKanban  bool
	MissingArchive bool
}

type LoadRepo struct{}

type CommitChanges struct{}

type CommitResult struct {
	URL          string
	OID          string
	Error        string
	Unauthorized bool
}

type ReposLoaded struct {
	Repos        []RepoOption
	Error        string
	Unauthorized bool
}

type SelectRepo struct {
	FullName string
}

type OpenInstallURL struct{}

type RepoSelectionSaved struct {
	Error string
}

type UpdateSearch struct {
	Value string
}

type AddFilter struct {
	Type  string
	Value string
}

type RemoveFilter struct {
	Index int
}

type ClearFilters struct{}

type OpenModal struct {
	Mode   ModalMode
	TaskID string
}

type CloseModal struct {
	Mode ModalMode
}

type UpdateFormField struct {
	Field string
	Value string
}

type UpdateCommitMessage struct {
	Value string
}

type ClearStatus struct {
	Seq int
}

type SetTagSuggestionsOpen struct {
	Open bool
}

type SelectTagSuggestion struct {
	Tag string
}

type AddFormSubtask struct {
	Text    string
	DueDate string
}

type ToggleFormSubtask struct {
	Index int
}

type UpdateFormSubtaskText struct {
	Index int
	Value string
}

type UpdateFormSubtaskDueDate struct {
	Index int
	Value string
}

type DeleteFormSubtask struct {
	Index int
}

type ConfirmCommit struct {
	Message string
}

type UpdateDetailSubtaskField struct {
	Field string
	Value string
}

type AddTaskSubtask struct {
	TaskID  string
	Text    string
	DueDate string
}

type ToggleTaskSubtask struct {
	TaskID string
	Index  int
}

type UpdateTaskSubtaskText struct {
	TaskID string
	Index  int
	Value  string
}

type UpdateTaskSubtaskDueDate struct {
	TaskID string
	Index  int
	Value  string
}

type DeleteTaskSubtask struct {
	TaskID string
	Index  int
}

type SaveTask struct{}

type DeleteTask struct {
	TaskID string
}

type ArchiveTask struct {
	TaskID string
}

type RestoreTask struct {
	TaskID string
}

type MoveTaskPosition struct {
	TaskID    string
	Direction int
}

type CloneTask struct {
	TaskID string
}

type AddColumn struct{}

type UpdateColumn struct {
	Index int
	Field string
	Value string
}

type DeleteColumn struct {
	Index int
}

type MoveColumn struct {
	Index     int
	Direction int
}

type UpdateArchiveSearch struct {
	Value string
}

type DragStartTask struct {
	TaskID string
}

type DragOverTask struct {
	TaskID   string
	ColumnID string
}

type DragOverColumn struct {
	ColumnID string
}

type DragEndTask struct{}

type DropOnTask struct {
	TargetTaskID string
	ColumnID     string
}

type DropOnColumn struct {
	ColumnID string
}

type OpenDetailFromTodo struct {
	TaskID string
}

type Logout struct{}

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
