//go:build js && wasm

package main

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
