//go:build js && wasm

package main

import "github.com/octoberswimmer/spectus/internal/merge"

// mergeKanban performs a 3-way merge of kanban content.
// It applies local changes (relative to base) onto the remote content.
func mergeKanban(base, local, remote string) (BoardConfig, []Task) {
	_, baseTasks := parseKanban(base)
	_, localTasks := parseKanban(local)
	remoteConfig, remoteTasks := parseKanban(remote)

	mergedMergeTasks := merge.MergeTaskLists(
		toMergeTasks(baseTasks),
		toMergeTasks(localTasks),
		toMergeTasks(remoteTasks),
	)

	return remoteConfig, fromMergeTasks(mergedMergeTasks)
}

// mergeArchive performs a 3-way merge of archive content.
func mergeArchive(base, local, remote string) []Task {
	baseTasks := parseArchive(base)
	localTasks := parseArchive(local)
	remoteTasks := parseArchive(remote)

	mergedMergeTasks := merge.MergeTaskLists(
		toMergeTasks(baseTasks),
		toMergeTasks(localTasks),
		toMergeTasks(remoteTasks),
	)

	return fromMergeTasks(mergedMergeTasks)
}

func toMergeTasks(tasks []Task) []merge.Task {
	result := make([]merge.Task, len(tasks))
	for i, t := range tasks {
		result[i] = merge.Task{
			ID:          t.ID,
			Title:       t.Title,
			Status:      t.Status,
			Category:    t.Category,
			Assignees:   t.Assignees,
			Tags:        t.Tags,
			Created:     t.Created,
			Modified:    t.Modified,
			Completed:   t.Completed,
			Description: t.Description,
			Subtasks:    toMergeSubtasks(t.Subtasks),
			Notes:       t.Notes,
		}
	}
	return result
}

func fromMergeTasks(tasks []merge.Task) []Task {
	result := make([]Task, len(tasks))
	for i, t := range tasks {
		result[i] = Task{
			ID:          t.ID,
			Title:       t.Title,
			Status:      t.Status,
			Category:    t.Category,
			Assignees:   t.Assignees,
			Tags:        t.Tags,
			Created:     t.Created,
			Modified:    t.Modified,
			Completed:   t.Completed,
			Description: t.Description,
			Subtasks:    fromMergeSubtasks(t.Subtasks),
			Notes:       t.Notes,
		}
	}
	return result
}

func toMergeSubtasks(subtasks []Subtask) []merge.Subtask {
	result := make([]merge.Subtask, len(subtasks))
	for i, s := range subtasks {
		result[i] = merge.Subtask{
			Completed: s.Completed,
			Text:      s.Text,
			DueDate:   s.DueDate,
		}
	}
	return result
}

func fromMergeSubtasks(subtasks []merge.Subtask) []Subtask {
	result := make([]Subtask, len(subtasks))
	for i, s := range subtasks {
		result[i] = Subtask{
			Completed: s.Completed,
			Text:      s.Text,
			DueDate:   s.DueDate,
		}
	}
	return result
}
