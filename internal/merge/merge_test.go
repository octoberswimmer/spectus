package merge

import (
	"testing"
)

func TestMergeString(t *testing.T) {
	tests := []struct {
		name           string
		base           string
		local          string
		remote         string
		expectedResult string
	}{
		{
			name:           "neither_changed",
			base:           "value",
			local:          "value",
			remote:         "value",
			expectedResult: "value",
		},
		{
			name:           "only_local_changed",
			base:           "original",
			local:          "local change",
			remote:         "original",
			expectedResult: "local change",
		},
		{
			name:           "only_remote_changed",
			base:           "original",
			local:          "original",
			remote:         "remote change",
			expectedResult: "remote change",
		},
		{
			name:           "both_changed_same_value",
			base:           "original",
			local:          "same",
			remote:         "same",
			expectedResult: "same",
		},
		{
			name:           "both_changed_different_values_prefers_local",
			base:           "original",
			local:          "local",
			remote:         "remote",
			expectedResult: "local",
		},
		{
			name:           "empty_base_only_local_set",
			base:           "",
			local:          "added",
			remote:         "",
			expectedResult: "added",
		},
		{
			name:           "empty_base_only_remote_set",
			base:           "",
			local:          "",
			remote:         "added",
			expectedResult: "added",
		},
		{
			name:           "local_clears_value",
			base:           "original",
			local:          "",
			remote:         "original",
			expectedResult: "",
		},
		{
			name:           "remote_clears_value",
			base:           "original",
			local:          "original",
			remote:         "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeString(tt.base, tt.local, tt.remote)
			if result != tt.expectedResult {
				t.Errorf("MergeString(%q, %q, %q) = %q, want %q",
					tt.base, tt.local, tt.remote, result, tt.expectedResult)
			}
		})
	}
}

func TestMergeStringSlice(t *testing.T) {
	tests := []struct {
		name           string
		base           []string
		local          []string
		remote         []string
		expectedResult []string
	}{
		{
			name:           "neither_changed",
			base:           []string{"a", "b"},
			local:          []string{"a", "b"},
			remote:         []string{"a", "b"},
			expectedResult: []string{"a", "b"},
		},
		{
			name:           "only_local_changed",
			base:           []string{"a", "b"},
			local:          []string{"a", "b", "c"},
			remote:         []string{"a", "b"},
			expectedResult: []string{"a", "b", "c"},
		},
		{
			name:           "only_remote_changed",
			base:           []string{"a", "b"},
			local:          []string{"a", "b"},
			remote:         []string{"a", "b", "c"},
			expectedResult: []string{"a", "b", "c"},
		},
		{
			name:           "both_changed_prefers_local",
			base:           []string{"a"},
			local:          []string{"a", "local"},
			remote:         []string{"a", "remote"},
			expectedResult: []string{"a", "local"},
		},
		{
			name:           "nil_slices",
			base:           nil,
			local:          nil,
			remote:         nil,
			expectedResult: nil,
		},
		{
			name:           "empty_to_non_empty_local",
			base:           []string{},
			local:          []string{"added"},
			remote:         []string{},
			expectedResult: []string{"added"},
		},
		{
			name:           "order_matters",
			base:           []string{"a", "b"},
			local:          []string{"b", "a"},
			remote:         []string{"a", "b"},
			expectedResult: []string{"b", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeStringSlice(tt.base, tt.local, tt.remote)
			if !slicesEqual(result, tt.expectedResult) {
				t.Errorf("MergeStringSlice() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTasksEqual(t *testing.T) {
	baseTask := Task{
		ID:          "TASK-001",
		Title:       "Test Task",
		Status:      "todo",
		Category:    "Backend",
		Assignees:   []string{"@user"},
		Tags:        []string{"bug"},
		Description: "Description",
		Notes:       "Notes",
		Subtasks: []Subtask{
			{Completed: false, Text: "Subtask 1", DueDate: ""},
		},
	}

	t.Run("identical_tasks_are_equal", func(t *testing.T) {
		other := baseTask
		if !TasksEqual(baseTask, other) {
			t.Error("identical tasks should be equal")
		}
	})

	t.Run("different_title", func(t *testing.T) {
		other := baseTask
		other.Title = "Different"
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different titles should not be equal")
		}
	})

	t.Run("different_status", func(t *testing.T) {
		other := baseTask
		other.Status = "done"
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different status should not be equal")
		}
	})

	t.Run("different_category", func(t *testing.T) {
		other := baseTask
		other.Category = "Frontend"
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different category should not be equal")
		}
	})

	t.Run("different_assignees", func(t *testing.T) {
		other := baseTask
		other.Assignees = []string{"@other"}
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different assignees should not be equal")
		}
	})

	t.Run("different_tags", func(t *testing.T) {
		other := baseTask
		other.Tags = []string{"feature"}
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different tags should not be equal")
		}
	})

	t.Run("different_description", func(t *testing.T) {
		other := baseTask
		other.Description = "Different desc"
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different description should not be equal")
		}
	})

	t.Run("different_notes", func(t *testing.T) {
		other := baseTask
		other.Notes = "Different notes"
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different notes should not be equal")
		}
	})

	t.Run("different_subtasks", func(t *testing.T) {
		other := baseTask
		other.Subtasks = []Subtask{{Completed: true, Text: "Subtask 1"}}
		if TasksEqual(baseTask, other) {
			t.Error("tasks with different subtasks should not be equal")
		}
	})

	t.Run("id_created_modified_not_compared", func(t *testing.T) {
		other := baseTask
		other.ID = "TASK-002"
		other.Created = "2024-01-01"
		other.Modified = "2024-01-02"
		if !TasksEqual(baseTask, other) {
			t.Error("ID, Created, and Modified fields should not affect equality")
		}
	})
}

func TestSubtasksEqual(t *testing.T) {
	t.Run("empty_slices_are_equal", func(t *testing.T) {
		if !SubtasksEqual(nil, nil) {
			t.Error("nil slices should be equal")
		}
		if !SubtasksEqual([]Subtask{}, []Subtask{}) {
			t.Error("empty slices should be equal")
		}
	})

	t.Run("different_lengths_not_equal", func(t *testing.T) {
		a := []Subtask{{Text: "one"}}
		b := []Subtask{{Text: "one"}, {Text: "two"}}
		if SubtasksEqual(a, b) {
			t.Error("slices with different lengths should not be equal")
		}
	})

	t.Run("different_completed_not_equal", func(t *testing.T) {
		a := []Subtask{{Completed: false, Text: "task"}}
		b := []Subtask{{Completed: true, Text: "task"}}
		if SubtasksEqual(a, b) {
			t.Error("subtasks with different completed status should not be equal")
		}
	})

	t.Run("different_text_not_equal", func(t *testing.T) {
		a := []Subtask{{Text: "task 1"}}
		b := []Subtask{{Text: "task 2"}}
		if SubtasksEqual(a, b) {
			t.Error("subtasks with different text should not be equal")
		}
	})

	t.Run("different_due_date_not_equal", func(t *testing.T) {
		a := []Subtask{{Text: "task", DueDate: "2024-01-01"}}
		b := []Subtask{{Text: "task", DueDate: "2024-01-02"}}
		if SubtasksEqual(a, b) {
			t.Error("subtasks with different due dates should not be equal")
		}
	})
}

func TestTaskMap(t *testing.T) {
	tasks := []Task{
		{ID: "TASK-001", Title: "First"},
		{ID: "TASK-002", Title: "Second"},
		{ID: "TASK-003", Title: "Third"},
	}

	m := TaskMap(tasks)

	if len(m) != 3 {
		t.Errorf("expected 3 entries, got %d", len(m))
	}

	if m["TASK-001"].Title != "First" {
		t.Error("TASK-001 not found or wrong title")
	}
	if m["TASK-002"].Title != "Second" {
		t.Error("TASK-002 not found or wrong title")
	}
	if m["TASK-003"].Title != "Third" {
		t.Error("TASK-003 not found or wrong title")
	}
}

func TestMergeTask(t *testing.T) {
	baseTask := Task{
		ID:          "TASK-001",
		Title:       "Original Title",
		Status:      "todo",
		Category:    "Backend",
		Assignees:   []string{"@user1"},
		Tags:        []string{"bug"},
		Created:     "2024-01-01T00:00:00Z",
		Modified:    "2024-01-01T00:00:00Z",
		Description: "Original description",
		Subtasks:    []Subtask{{Text: "Original subtask"}},
		Notes:       "Original notes",
	}

	t.Run("no_changes_returns_base", func(t *testing.T) {
		result := MergeTask(baseTask, baseTask, baseTask)
		if result.Title != baseTask.Title {
			t.Error("expected base title")
		}
		if result.Status != baseTask.Status {
			t.Error("expected base status")
		}
	})

	t.Run("local_title_change_preserved", func(t *testing.T) {
		local := baseTask
		local.Title = "Local Title"
		result := MergeTask(baseTask, local, baseTask)
		if result.Title != "Local Title" {
			t.Errorf("expected 'Local Title', got %q", result.Title)
		}
	})

	t.Run("remote_status_change_preserved", func(t *testing.T) {
		remote := baseTask
		remote.Status = "done"
		result := MergeTask(baseTask, baseTask, remote)
		if result.Status != "done" {
			t.Errorf("expected 'done', got %q", result.Status)
		}
	})

	t.Run("different_properties_both_preserved", func(t *testing.T) {
		local := baseTask
		local.Title = "NEW NAME!"
		remote := baseTask
		remote.Status = "done"
		result := MergeTask(baseTask, local, remote)
		if result.Title != "NEW NAME!" {
			t.Errorf("expected local title 'NEW NAME!', got %q", result.Title)
		}
		if result.Status != "done" {
			t.Errorf("expected remote status 'done', got %q", result.Status)
		}
	})

	t.Run("conflict_on_same_property_prefers_local", func(t *testing.T) {
		local := baseTask
		local.Title = "Local Title"
		remote := baseTask
		remote.Title = "Remote Title"
		result := MergeTask(baseTask, local, remote)
		if result.Title != "Local Title" {
			t.Errorf("expected 'Local Title' on conflict, got %q", result.Title)
		}
	})

	t.Run("modified_uses_most_recent", func(t *testing.T) {
		local := baseTask
		local.Modified = "2024-01-02T00:00:00Z"
		remote := baseTask
		remote.Modified = "2024-01-03T00:00:00Z"
		result := MergeTask(baseTask, local, remote)
		if result.Modified != "2024-01-03T00:00:00Z" {
			t.Errorf("expected remote modified time, got %q", result.Modified)
		}
	})

	t.Run("created_always_uses_local", func(t *testing.T) {
		local := baseTask
		local.Created = "2024-01-01T00:00:00Z"
		remote := baseTask
		remote.Created = "2024-01-02T00:00:00Z"
		result := MergeTask(baseTask, local, remote)
		if result.Created != "2024-01-01T00:00:00Z" {
			t.Errorf("expected local created time, got %q", result.Created)
		}
	})

	t.Run("local_subtask_change_preserved", func(t *testing.T) {
		local := baseTask
		local.Subtasks = []Subtask{{Text: "New subtask", Completed: true}}
		result := MergeTask(baseTask, local, baseTask)
		if len(result.Subtasks) != 1 || result.Subtasks[0].Text != "New subtask" {
			t.Error("expected local subtasks to be preserved")
		}
	})

	t.Run("remote_subtask_change_when_local_unchanged", func(t *testing.T) {
		remote := baseTask
		remote.Subtasks = []Subtask{{Text: "Remote subtask"}}
		result := MergeTask(baseTask, baseTask, remote)
		if len(result.Subtasks) != 1 || result.Subtasks[0].Text != "Remote subtask" {
			t.Error("expected remote subtasks when local unchanged")
		}
	})

	t.Run("assignees_local_change", func(t *testing.T) {
		local := baseTask
		local.Assignees = []string{"@user1", "@user2"}
		result := MergeTask(baseTask, local, baseTask)
		if len(result.Assignees) != 2 {
			t.Errorf("expected 2 assignees, got %d", len(result.Assignees))
		}
	})

	t.Run("tags_remote_change", func(t *testing.T) {
		remote := baseTask
		remote.Tags = []string{"bug", "urgent"}
		result := MergeTask(baseTask, baseTask, remote)
		if len(result.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(result.Tags))
		}
	})
}

func TestMergeTaskLists(t *testing.T) {
	baseTask := Task{
		ID:       "TASK-001",
		Title:    "First Card",
		Status:   "todo",
		Category: "Backend",
	}

	t.Run("no_changes", func(t *testing.T) {
		base := []Task{baseTask}
		tasks := MergeTaskLists(base, base, base)
		if len(tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Title != "First Card" {
			t.Errorf("expected 'First Card', got %q", tasks[0].Title)
		}
	})

	t.Run("local_adds_task", func(t *testing.T) {
		base := []Task{baseTask}
		newTask := Task{ID: "TASK-002", Title: "New Local Task", Status: "todo"}
		local := []Task{baseTask, newTask}
		tasks := MergeTaskLists(base, local, base)
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(tasks))
		}
		found := false
		for _, task := range tasks {
			if task.ID == "TASK-002" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find TASK-002")
		}
	})

	t.Run("remote_adds_task", func(t *testing.T) {
		base := []Task{baseTask}
		newTask := Task{ID: "TASK-003", Title: "New Remote Task", Status: "todo"}
		remote := []Task{baseTask, newTask}
		tasks := MergeTaskLists(base, base, remote)
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(tasks))
		}
	})

	t.Run("both_add_different_tasks", func(t *testing.T) {
		base := []Task{baseTask}
		localNew := Task{ID: "TASK-002", Title: "Local Task", Status: "todo"}
		remoteNew := Task{ID: "TASK-003", Title: "Remote Task", Status: "todo"}
		local := []Task{baseTask, localNew}
		remote := []Task{baseTask, remoteNew}
		tasks := MergeTaskLists(base, local, remote)
		if len(tasks) != 3 {
			t.Errorf("expected 3 tasks, got %d", len(tasks))
		}
	})

	t.Run("local_deletes_task_remote_unchanged", func(t *testing.T) {
		base := []Task{baseTask}
		local := []Task{}
		tasks := MergeTaskLists(base, local, base)
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks after local delete, got %d", len(tasks))
		}
	})

	t.Run("remote_deletes_task_local_modified", func(t *testing.T) {
		base := []Task{baseTask}
		modifiedTask := baseTask
		modifiedTask.Title = "Modified Local Title"
		local := []Task{modifiedTask}
		remote := []Task{}
		tasks := MergeTaskLists(base, local, remote)
		if len(tasks) != 1 {
			t.Errorf("expected 1 task (local modified wins over remote delete), got %d", len(tasks))
		}
		if tasks[0].Title != "Modified Local Title" {
			t.Errorf("expected 'Modified Local Title', got %q", tasks[0].Title)
		}
	})

	t.Run("remote_deletes_task_local_unchanged", func(t *testing.T) {
		base := []Task{baseTask}
		local := base
		remote := []Task{}
		tasks := MergeTaskLists(base, local, remote)
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks (remote delete wins when local unchanged), got %d", len(tasks))
		}
	})

	t.Run("local_deletes_task_remote_modified", func(t *testing.T) {
		base := []Task{baseTask}
		local := []Task{}
		modifiedTask := baseTask
		modifiedTask.Title = "Modified Remote Title"
		remote := []Task{modifiedTask}
		tasks := MergeTaskLists(base, local, remote)
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks (local delete wins), got %d", len(tasks))
		}
	})

	t.Run("merge_different_properties_of_same_task", func(t *testing.T) {
		base := []Task{baseTask}
		localTask := baseTask
		localTask.Title = "NEW NAME!"
		remoteTask := baseTask
		remoteTask.Status = "done"
		local := []Task{localTask}
		remote := []Task{remoteTask}
		tasks := MergeTaskLists(base, local, remote)
		if len(tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(tasks))
		}
		task := tasks[0]
		if task.Title != "NEW NAME!" {
			t.Errorf("expected local title 'NEW NAME!', got %q", task.Title)
		}
		if task.Status != "done" {
			t.Errorf("expected remote status 'done', got %q", task.Status)
		}
	})

	t.Run("conflict_on_same_property_prefers_local", func(t *testing.T) {
		base := []Task{baseTask}
		localTask := baseTask
		localTask.Title = "Local Title"
		remoteTask := baseTask
		remoteTask.Title = "Remote Title"
		local := []Task{localTask}
		remote := []Task{remoteTask}
		tasks := MergeTaskLists(base, local, remote)
		if len(tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Title != "Local Title" {
			t.Errorf("expected 'Local Title', got %q", tasks[0].Title)
		}
	})

	t.Run("empty_base_both_add_same_task_id", func(t *testing.T) {
		base := []Task{}
		localTask := Task{ID: "TASK-001", Title: "Local Version", Status: "todo"}
		remoteTask := Task{ID: "TASK-001", Title: "Remote Version", Status: "done"}
		local := []Task{localTask}
		remote := []Task{remoteTask}
		tasks := MergeTaskLists(base, local, remote)
		// Both are "new" since not in base - local should be included, remote ignored
		if len(tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Title != "Local Version" {
			t.Errorf("expected 'Local Version', got %q", tasks[0].Title)
		}
	})
}
