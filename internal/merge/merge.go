package merge

import "slices"

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
	ID        string
	Completed bool
	Text      string
	DueDate   string
}

// MergeTaskLists performs a 3-way merge of task lists.
// It applies local changes (relative to base) onto the remote content.
func MergeTaskLists(baseTasks, localTasks, remoteTasks []Task) []Task {
	baseMap := TaskMap(baseTasks)
	remoteMap := TaskMap(remoteTasks)

	processed := make(map[string]bool)
	var mergedTasks []Task

	// Process all tasks from local (includes modified and new tasks)
	for _, localTask := range localTasks {
		processed[localTask.ID] = true
		baseTask, inBase := baseMap[localTask.ID]
		remoteTask, inRemote := remoteMap[localTask.ID]

		if !inBase {
			// New local task - add it
			mergedTasks = append(mergedTasks, localTask)
		} else if !inRemote {
			// Task was deleted remotely - keep local if modified, otherwise delete
			if !TasksEqual(baseTask, localTask) {
				mergedTasks = append(mergedTasks, localTask)
			}
			// If local == base and remote deleted, let it stay deleted
		} else {
			// Task exists in all three - merge properties
			merged := MergeTask(baseTask, localTask, remoteTask)
			mergedTasks = append(mergedTasks, merged)
		}
	}

	// Add tasks that exist in remote but not in local
	for _, remoteTask := range remoteTasks {
		if processed[remoteTask.ID] {
			continue
		}
		_, inBase := baseMap[remoteTask.ID]
		if !inBase {
			// New remote task - add it
			mergedTasks = append(mergedTasks, remoteTask)
		}
		// If task was in base but not in local, it was deleted locally - don't add
	}

	return mergedTasks
}

func TaskMap(tasks []Task) map[string]Task {
	m := make(map[string]Task)
	for _, t := range tasks {
		m[t.ID] = t
	}
	return m
}

// MergeTask merges individual task properties using 3-way merge logic.
// For each property: if only one side changed it, use that change.
// If both changed, prefer local (user's intent).
func MergeTask(base, local, remote Task) Task {
	merged := Task{ID: local.ID}

	// Title
	merged.Title = MergeString(base.Title, local.Title, remote.Title)

	// Status (column)
	merged.Status = MergeString(base.Status, local.Status, remote.Status)

	// Category
	merged.Category = MergeString(base.Category, local.Category, remote.Category)

	// Assignees
	merged.Assignees = MergeStringSlice(base.Assignees, local.Assignees, remote.Assignees)

	// Tags
	merged.Tags = MergeStringSlice(base.Tags, local.Tags, remote.Tags)

	// Created (should never change, use local)
	merged.Created = local.Created

	// Modified (use most recent)
	merged.Modified = local.Modified
	if remote.Modified > local.Modified {
		merged.Modified = remote.Modified
	}

	// Completed
	merged.Completed = MergeString(base.Completed, local.Completed, remote.Completed)

	// Description
	merged.Description = MergeString(base.Description, local.Description, remote.Description)

	// Subtasks (complex, for now prefer local if changed)
	if !SubtasksEqual(base.Subtasks, local.Subtasks) {
		merged.Subtasks = local.Subtasks
	} else {
		merged.Subtasks = remote.Subtasks
	}

	// Notes
	merged.Notes = MergeString(base.Notes, local.Notes, remote.Notes)

	return merged
}

func MergeString(base, local, remote string) string {
	localChanged := local != base
	remoteChanged := remote != base

	if localChanged && !remoteChanged {
		return local
	}
	if !localChanged && remoteChanged {
		return remote
	}
	if localChanged && remoteChanged {
		// Both changed - prefer local (user's intent)
		return local
	}
	// Neither changed
	return base
}

func MergeStringSlice(base, local, remote []string) []string {
	localChanged := !slices.Equal(base, local)
	remoteChanged := !slices.Equal(base, remote)

	if localChanged && !remoteChanged {
		return local
	}
	if !localChanged && remoteChanged {
		return remote
	}
	if localChanged && remoteChanged {
		return local
	}
	return base
}

func TasksEqual(a, b Task) bool {
	return a.Title == b.Title &&
		a.Status == b.Status &&
		a.Category == b.Category &&
		slices.Equal(a.Assignees, b.Assignees) &&
		slices.Equal(a.Tags, b.Tags) &&
		a.Completed == b.Completed &&
		a.Description == b.Description &&
		SubtasksEqual(a.Subtasks, b.Subtasks) &&
		a.Notes == b.Notes
}

func SubtasksEqual(a, b []Subtask) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID ||
			a[i].Completed != b[i].Completed ||
			a[i].Text != b[i].Text ||
			a[i].DueDate != b[i].DueDate {
			return false
		}
	}
	return true
}

// DeleteTaskByID removes a task from either the tasks or archived slice.
// It first checks tasks, then archived. Returns the modified slices and
// whether a task was deleted.
func DeleteTaskByID(taskID string, tasks, archived []Task) ([]Task, []Task, bool) {
	for i, task := range tasks {
		if task.ID == taskID {
			return append(tasks[:i], tasks[i+1:]...), archived, true
		}
	}
	for i, task := range archived {
		if task.ID == taskID {
			return tasks, append(archived[:i], archived[i+1:]...), true
		}
	}
	return tasks, archived, false
}

// Identifiable is any type that has an ID field.
type Identifiable interface {
	GetID() string
}

// DeleteByID removes an item from either the first or second slice.
// It first checks the first slice, then the second. Returns the modified
// slices and whether an item was deleted.
func DeleteByID[T Identifiable](id string, first, second []T) ([]T, []T, bool) {
	for i, item := range first {
		if item.GetID() == id {
			return append(first[:i], first[i+1:]...), second, true
		}
	}
	for i, item := range second {
		if item.GetID() == id {
			return first, append(second[:i], second[i+1:]...), true
		}
	}
	return first, second, false
}

// GetID implements Identifiable for Task.
func (t Task) GetID() string {
	return t.ID
}
