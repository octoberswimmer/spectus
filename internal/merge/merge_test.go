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

// TestConflictResolutionScenarios tests real-world conflict scenarios
func TestConflictResolutionScenarios(t *testing.T) {
	// Base state: task in "To Do" column with original title
	baseTask := Task{
		ID:          "TASK-MLFM5M12M3",
		Title:       "first card",
		Status:      "todo",
		Category:    "Backend",
		Assignees:   []string{"@user"},
		Tags:        []string{"bug"},
		Created:     "2026-02-09T00:00:00Z",
		Modified:    "2026-02-09T00:00:00Z",
		Description: "Original description",
	}

	t.Run("local_renames_remote_moves_column", func(t *testing.T) {
		// User scenario: local renames task, remote moves to different column
		base := []Task{baseTask}

		// Local: renamed title
		localTask := baseTask
		localTask.Title = "Another NEW NAME!"
		local := []Task{localTask}

		// Remote: moved to "In Progress" column
		remoteTask := baseTask
		remoteTask.Status = "in-progress"
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		task := tasks[0]
		if task.Title != "Another NEW NAME!" {
			t.Errorf("expected local title 'Another NEW NAME!', got %q", task.Title)
		}
		if task.Status != "in-progress" {
			t.Errorf("expected remote status 'in-progress', got %q", task.Status)
		}
		// Other properties should remain unchanged
		if task.Category != "Backend" {
			t.Errorf("expected category 'Backend', got %q", task.Category)
		}
	})

	t.Run("local_adds_description_remote_adds_tags", func(t *testing.T) {
		base := []Task{baseTask}

		localTask := baseTask
		localTask.Description = "Updated description with details"
		local := []Task{localTask}

		remoteTask := baseTask
		remoteTask.Tags = []string{"bug", "urgent", "priority"}
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		task := tasks[0]
		if task.Description != "Updated description with details" {
			t.Errorf("expected local description, got %q", task.Description)
		}
		if len(task.Tags) != 3 || task.Tags[0] != "bug" {
			t.Errorf("expected remote tags [bug urgent priority], got %v", task.Tags)
		}
	})

	t.Run("local_assigns_user_remote_changes_category", func(t *testing.T) {
		base := []Task{baseTask}

		localTask := baseTask
		localTask.Assignees = []string{"@user", "@newuser"}
		local := []Task{localTask}

		remoteTask := baseTask
		remoteTask.Category = "Frontend"
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		task := tasks[0]
		if len(task.Assignees) != 2 {
			t.Errorf("expected 2 assignees, got %d", len(task.Assignees))
		}
		if task.Category != "Frontend" {
			t.Errorf("expected remote category 'Frontend', got %q", task.Category)
		}
	})

	t.Run("multiple_tasks_different_changes", func(t *testing.T) {
		task1 := Task{ID: "TASK-001", Title: "Task One", Status: "todo"}
		task2 := Task{ID: "TASK-002", Title: "Task Two", Status: "todo"}
		task3 := Task{ID: "TASK-003", Title: "Task Three", Status: "todo"}
		base := []Task{task1, task2, task3}

		// Local: rename task1, delete task2, add task4
		localTask1 := task1
		localTask1.Title = "Task One Renamed"
		task4 := Task{ID: "TASK-004", Title: "New Local Task", Status: "todo"}
		local := []Task{localTask1, task3, task4}

		// Remote: move task1 to done, rename task3, add task5
		remoteTask1 := task1
		remoteTask1.Status = "done"
		remoteTask3 := task3
		remoteTask3.Title = "Task Three Updated"
		task5 := Task{ID: "TASK-005", Title: "New Remote Task", Status: "in-progress"}
		remote := []Task{remoteTask1, task2, remoteTask3, task5}

		tasks := MergeTaskLists(base, local, remote)

		// Expected: 4 tasks
		// - task1: renamed locally + moved remotely
		// - task2: deleted locally (local delete wins)
		// - task3: renamed remotely (local unchanged)
		// - task4: added locally
		// - task5: added remotely
		if len(tasks) != 4 {
			t.Fatalf("expected 4 tasks, got %d", len(tasks))
		}

		taskMap := make(map[string]Task)
		for _, task := range tasks {
			taskMap[task.ID] = task
		}

		// Task1: local title + remote status
		if taskMap["TASK-001"].Title != "Task One Renamed" {
			t.Errorf("task1: expected 'Task One Renamed', got %q", taskMap["TASK-001"].Title)
		}
		if taskMap["TASK-001"].Status != "done" {
			t.Errorf("task1: expected status 'done', got %q", taskMap["TASK-001"].Status)
		}

		// Task2: should be deleted (local delete)
		if _, exists := taskMap["TASK-002"]; exists {
			t.Error("task2 should have been deleted")
		}

		// Task3: remote title (local unchanged)
		if taskMap["TASK-003"].Title != "Task Three Updated" {
			t.Errorf("task3: expected 'Task Three Updated', got %q", taskMap["TASK-003"].Title)
		}

		// Task4: added locally
		if _, exists := taskMap["TASK-004"]; !exists {
			t.Error("task4 should exist (added locally)")
		}

		// Task5: added remotely
		if _, exists := taskMap["TASK-005"]; !exists {
			t.Error("task5 should exist (added remotely)")
		}
	})

	t.Run("cascading_merge_simulation", func(t *testing.T) {
		// Simulates what happens when a retry commit also fails
		// First merge: base A, local B, remote C -> M1
		// Second merge: base C, local M1, remote D -> M2

		taskA := Task{ID: "TASK-001", Title: "Original", Status: "todo", Category: "Backend"}
		taskB := Task{ID: "TASK-001", Title: "User Renamed", Status: "todo", Category: "Backend"}
		taskC := Task{ID: "TASK-001", Title: "Original", Status: "in-progress", Category: "Backend"}

		// First merge
		merged1 := MergeTaskLists([]Task{taskA}, []Task{taskB}, []Task{taskC})
		if len(merged1) != 1 {
			t.Fatalf("first merge: expected 1 task, got %d", len(merged1))
		}
		m1 := merged1[0]
		if m1.Title != "User Renamed" || m1.Status != "in-progress" {
			t.Errorf("first merge: expected title='User Renamed' status='in-progress', got title=%q status=%q", m1.Title, m1.Status)
		}

		// Second merge: someone else changed category while we were merging
		taskD := Task{ID: "TASK-001", Title: "Original", Status: "in-progress", Category: "Frontend"}

		merged2 := MergeTaskLists([]Task{taskC}, []Task{m1}, []Task{taskD})
		if len(merged2) != 1 {
			t.Fatalf("second merge: expected 1 task, got %d", len(merged2))
		}
		m2 := merged2[0]

		// Should have: title from m1 (changed from C), status same, category from D (changed from C)
		if m2.Title != "User Renamed" {
			t.Errorf("second merge: expected title 'User Renamed', got %q", m2.Title)
		}
		if m2.Status != "in-progress" {
			t.Errorf("second merge: expected status 'in-progress', got %q", m2.Status)
		}
		if m2.Category != "Frontend" {
			t.Errorf("second merge: expected category 'Frontend', got %q", m2.Category)
		}
	})

	t.Run("local_completes_subtask_remote_adds_subtask", func(t *testing.T) {
		taskWithSubtasks := Task{
			ID:       "TASK-001",
			Title:    "Task with subtasks",
			Status:   "todo",
			Subtasks: []Subtask{{Text: "Subtask 1", Completed: false}},
		}
		base := []Task{taskWithSubtasks}

		// Local: complete the subtask
		localTask := taskWithSubtasks
		localTask.Subtasks = []Subtask{{Text: "Subtask 1", Completed: true}}
		local := []Task{localTask}

		// Remote: add a new subtask (subtasks changed, so different)
		remoteTask := taskWithSubtasks
		remoteTask.Subtasks = []Subtask{
			{Text: "Subtask 1", Completed: false},
			{Text: "Subtask 2", Completed: false},
		}
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		// Both changed subtasks, so local wins
		task := tasks[0]
		if len(task.Subtasks) != 1 {
			t.Errorf("expected 1 subtask (local wins), got %d", len(task.Subtasks))
		}
		if !task.Subtasks[0].Completed {
			t.Error("expected subtask to be completed (local change)")
		}
	})

	t.Run("notes_merge", func(t *testing.T) {
		taskWithNotes := Task{
			ID:     "TASK-001",
			Title:  "Task",
			Status: "todo",
			Notes:  "Original notes",
		}
		base := []Task{taskWithNotes}

		// Local: update notes
		localTask := taskWithNotes
		localTask.Notes = "Updated notes by local"
		local := []Task{localTask}

		// Remote: unchanged
		remote := []Task{taskWithNotes}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		if tasks[0].Notes != "Updated notes by local" {
			t.Errorf("expected local notes, got %q", tasks[0].Notes)
		}
	})

	t.Run("completed_timestamp_merge", func(t *testing.T) {
		task := Task{
			ID:        "TASK-001",
			Title:     "Task",
			Status:    "todo",
			Completed: "",
		}
		base := []Task{task}

		// Local: mark as complete
		localTask := task
		localTask.Status = "done"
		localTask.Completed = "2026-02-09T10:00:00Z"
		local := []Task{localTask}

		// Remote: also mark as complete at different time
		remoteTask := task
		remoteTask.Status = "done"
		remoteTask.Completed = "2026-02-09T09:00:00Z"
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		// Both changed Completed, so local wins
		if tasks[0].Completed != "2026-02-09T10:00:00Z" {
			t.Errorf("expected local completed time, got %q", tasks[0].Completed)
		}
	})
}

// TestSessionRestoreScenarios tests merge scenarios that occur during session restoration
// when local changes were saved to localStorage and remote has changed while session was expired
func TestSessionRestoreScenarios(t *testing.T) {
	t.Run("session_restore_remote_unchanged", func(t *testing.T) {
		// User made changes, session expired, but no one else touched the repo
		// Base == Remote, so local changes should be preserved exactly
		baseTask := Task{
			ID:     "TASK-001",
			Title:  "Original Task",
			Status: "todo",
		}
		base := []Task{baseTask}

		localTask := baseTask
		localTask.Title = "User's Local Changes"
		localTask.Status = "in-progress"
		local := []Task{localTask}

		// Remote is same as base (no changes while session was expired)
		remote := base

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Title != "User's Local Changes" {
			t.Errorf("expected local title, got %q", tasks[0].Title)
		}
		if tasks[0].Status != "in-progress" {
			t.Errorf("expected local status 'in-progress', got %q", tasks[0].Status)
		}
	})

	t.Run("session_restore_remote_changed_different_task", func(t *testing.T) {
		// User edited task1, remote added task2 while session expired
		task1 := Task{ID: "TASK-001", Title: "Task 1", Status: "todo"}
		base := []Task{task1}

		localTask1 := task1
		localTask1.Title = "Task 1 Edited"
		local := []Task{localTask1}

		task2 := Task{ID: "TASK-002", Title: "New Remote Task", Status: "todo"}
		remote := []Task{task1, task2}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(tasks))
		}

		taskMap := make(map[string]Task)
		for _, task := range tasks {
			taskMap[task.ID] = task
		}

		if taskMap["TASK-001"].Title != "Task 1 Edited" {
			t.Errorf("expected local edits preserved, got %q", taskMap["TASK-001"].Title)
		}
		if _, exists := taskMap["TASK-002"]; !exists {
			t.Error("expected remote task to be included")
		}
	})

	t.Run("session_restore_remote_changed_same_task", func(t *testing.T) {
		// User edited task, collaborator also edited same task while session expired
		baseTask := Task{
			ID:          "TASK-001",
			Title:       "Original Title",
			Status:      "todo",
			Description: "Original desc",
		}
		base := []Task{baseTask}

		// Local: changed title
		localTask := baseTask
		localTask.Title = "My New Title"
		local := []Task{localTask}

		// Remote: changed description (different property)
		remoteTask := baseTask
		remoteTask.Description = "Collaborator's description"
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		// Both changes should be preserved
		if tasks[0].Title != "My New Title" {
			t.Errorf("expected local title, got %q", tasks[0].Title)
		}
		if tasks[0].Description != "Collaborator's description" {
			t.Errorf("expected remote description, got %q", tasks[0].Description)
		}
	})

	t.Run("session_restore_local_added_remote_added", func(t *testing.T) {
		// Both user and collaborator added new tasks while session was expired
		base := []Task{}

		localTask := Task{ID: "TASK-LOCAL", Title: "My New Task", Status: "todo"}
		local := []Task{localTask}

		remoteTask := Task{ID: "TASK-REMOTE", Title: "Collaborator's Task", Status: "todo"}
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(tasks))
		}

		taskMap := make(map[string]Task)
		for _, task := range tasks {
			taskMap[task.ID] = task
		}

		if _, exists := taskMap["TASK-LOCAL"]; !exists {
			t.Error("expected local task to be included")
		}
		if _, exists := taskMap["TASK-REMOTE"]; !exists {
			t.Error("expected remote task to be included")
		}
	})

	t.Run("session_restore_local_deleted_remote_modified", func(t *testing.T) {
		// User deleted a task, but collaborator modified it while session expired
		// Local delete should win (user's intent)
		baseTask := Task{ID: "TASK-001", Title: "Original", Status: "todo"}
		base := []Task{baseTask}

		local := []Task{} // User deleted

		remoteTask := baseTask
		remoteTask.Title = "Collaborator Modified This"
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks (local delete wins), got %d", len(tasks))
		}
	})

	t.Run("session_restore_conflict_same_property", func(t *testing.T) {
		// Both user and collaborator changed the same property
		// Local should win (user's most recent intent)
		baseTask := Task{ID: "TASK-001", Title: "Original", Status: "todo"}
		base := []Task{baseTask}

		localTask := baseTask
		localTask.Title = "User's Title"
		local := []Task{localTask}

		remoteTask := baseTask
		remoteTask.Title = "Collaborator's Title"
		remote := []Task{remoteTask}

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Title != "User's Title" {
			t.Errorf("expected local title on conflict, got %q", tasks[0].Title)
		}
	})

	t.Run("session_restore_subtask_changes", func(t *testing.T) {
		// User completed a subtask, remote is unchanged
		baseTask := Task{
			ID:     "TASK-001",
			Title:  "Task",
			Status: "todo",
			Subtasks: []Subtask{
				{Text: "Subtask 1", Completed: false},
				{Text: "Subtask 2", Completed: false},
			},
		}
		base := []Task{baseTask}

		localTask := baseTask
		localTask.Subtasks = []Subtask{
			{Text: "Subtask 1", Completed: true}, // Completed this one
			{Text: "Subtask 2", Completed: false},
		}
		local := []Task{localTask}

		remote := base // Unchanged

		tasks := MergeTaskLists(base, local, remote)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if !tasks[0].Subtasks[0].Completed {
			t.Error("expected first subtask to be completed")
		}
	})

	t.Run("session_restore_long_session_many_remote_changes", func(t *testing.T) {
		// Simulates a long session expiration where many remote changes happened
		task1 := Task{ID: "TASK-001", Title: "Task 1", Status: "todo"}
		task2 := Task{ID: "TASK-002", Title: "Task 2", Status: "todo"}
		base := []Task{task1, task2}

		// Local: only modified task1's title
		localTask1 := task1
		localTask1.Title = "My Edit to Task 1"
		local := []Task{localTask1, task2}

		// Remote: task1 moved to done, task2 archived (deleted), task3 and task4 added
		remoteTask1 := task1
		remoteTask1.Status = "done"
		task3 := Task{ID: "TASK-003", Title: "New Task 3", Status: "todo"}
		task4 := Task{ID: "TASK-004", Title: "New Task 4", Status: "in-progress"}
		remote := []Task{remoteTask1, task3, task4} // task2 removed

		tasks := MergeTaskLists(base, local, remote)

		// Expected:
		// - task1: local title + remote status
		// - task2: kept (local didn't delete it, but remote did - local unchanged so remote delete wins? Actually no, if local has task and is unchanged, remote delete wins)
		// - task3: added from remote
		// - task4: added from remote

		taskMap := make(map[string]Task)
		for _, task := range tasks {
			taskMap[task.ID] = task
		}

		// Task1: merged
		if taskMap["TASK-001"].Title != "My Edit to Task 1" {
			t.Errorf("task1: expected local title, got %q", taskMap["TASK-001"].Title)
		}
		if taskMap["TASK-001"].Status != "done" {
			t.Errorf("task1: expected remote status 'done', got %q", taskMap["TASK-001"].Status)
		}

		// Task2: remote deleted, local unchanged -> deleted
		if _, exists := taskMap["TASK-002"]; exists {
			t.Error("task2 should be deleted (remote delete, local unchanged)")
		}

		// Task3 and Task4: added from remote
		if _, exists := taskMap["TASK-003"]; !exists {
			t.Error("task3 should exist (added remotely)")
		}
		if _, exists := taskMap["TASK-004"]; !exists {
			t.Error("task4 should exist (added remotely)")
		}
	})
}

// TestMergeTaskAllProperties tests merging when all properties are independently changed
func TestMergeTaskAllProperties(t *testing.T) {
	base := Task{
		ID:          "TASK-001",
		Title:       "Base Title",
		Status:      "todo",
		Category:    "Backend",
		Assignees:   []string{"@user1"},
		Tags:        []string{"tag1"},
		Created:     "2026-01-01T00:00:00Z",
		Modified:    "2026-01-01T00:00:00Z",
		Completed:   "",
		Description: "Base description",
		Subtasks:    []Subtask{{Text: "Base subtask", Completed: false}},
		Notes:       "Base notes",
	}

	// Local changes some properties
	local := Task{
		ID:          "TASK-001",
		Title:       "Local Title",           // changed
		Status:      "todo",                  // unchanged
		Category:    "Backend",               // unchanged
		Assignees:   []string{"@user1"},      // unchanged
		Tags:        []string{"tag1", "new"}, // changed
		Created:     "2026-01-01T00:00:00Z",
		Modified:    "2026-01-02T00:00:00Z",
		Completed:   "",
		Description: "Base description",                                  // unchanged
		Subtasks:    []Subtask{{Text: "Base subtask", Completed: false}}, // unchanged
		Notes:       "Local notes",                                       // changed
	}

	// Remote changes other properties
	remote := Task{
		ID:          "TASK-001",
		Title:       "Base Title",                 // unchanged
		Status:      "in-progress",                // changed
		Category:    "Frontend",                   // changed
		Assignees:   []string{"@user1", "@user2"}, // changed
		Tags:        []string{"tag1"},             // unchanged
		Created:     "2026-01-01T00:00:00Z",
		Modified:    "2026-01-03T00:00:00Z",
		Completed:   "",
		Description: "Remote description",                                // changed
		Subtasks:    []Subtask{{Text: "Base subtask", Completed: false}}, // unchanged
		Notes:       "Base notes",                                        // unchanged
	}

	result := MergeTask(base, local, remote)

	// Title: local changed, remote unchanged -> local
	if result.Title != "Local Title" {
		t.Errorf("Title: expected 'Local Title', got %q", result.Title)
	}

	// Status: local unchanged, remote changed -> remote
	if result.Status != "in-progress" {
		t.Errorf("Status: expected 'in-progress', got %q", result.Status)
	}

	// Category: local unchanged, remote changed -> remote
	if result.Category != "Frontend" {
		t.Errorf("Category: expected 'Frontend', got %q", result.Category)
	}

	// Assignees: local unchanged, remote changed -> remote
	if len(result.Assignees) != 2 {
		t.Errorf("Assignees: expected 2, got %d", len(result.Assignees))
	}

	// Tags: local changed, remote unchanged -> local
	if len(result.Tags) != 2 || result.Tags[1] != "new" {
		t.Errorf("Tags: expected [tag1 new], got %v", result.Tags)
	}

	// Description: local unchanged, remote changed -> remote
	if result.Description != "Remote description" {
		t.Errorf("Description: expected 'Remote description', got %q", result.Description)
	}

	// Notes: local changed, remote unchanged -> local
	if result.Notes != "Local notes" {
		t.Errorf("Notes: expected 'Local notes', got %q", result.Notes)
	}

	// Modified: use most recent
	if result.Modified != "2026-01-03T00:00:00Z" {
		t.Errorf("Modified: expected remote time (most recent), got %q", result.Modified)
	}
}

func TestDeleteTaskByID(t *testing.T) {
	t.Run("deletes_task_from_tasks_list", func(t *testing.T) {
		tasks := []Task{
			{ID: "TASK-001", Title: "Task 1"},
			{ID: "TASK-002", Title: "Task 2"},
			{ID: "TASK-003", Title: "Task 3"},
		}
		archived := []Task{}

		newTasks, newArchived, deleted := DeleteTaskByID("TASK-002", tasks, archived)

		if !deleted {
			t.Error("expected deleted to be true")
		}
		if len(newTasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(newTasks))
		}
		for _, task := range newTasks {
			if task.ID == "TASK-002" {
				t.Error("TASK-002 should have been deleted")
			}
		}
		if len(newArchived) != 0 {
			t.Errorf("expected 0 archived tasks, got %d", len(newArchived))
		}
	})

	t.Run("deletes_task_from_archived_list", func(t *testing.T) {
		tasks := []Task{}
		archived := []Task{
			{ID: "TASK-001", Title: "Archived 1"},
			{ID: "TASK-002", Title: "Archived 2"},
			{ID: "TASK-003", Title: "Archived 3"},
		}

		newTasks, newArchived, deleted := DeleteTaskByID("TASK-002", tasks, archived)

		if !deleted {
			t.Error("expected deleted to be true")
		}
		if len(newArchived) != 2 {
			t.Errorf("expected 2 archived tasks, got %d", len(newArchived))
		}
		for _, task := range newArchived {
			if task.ID == "TASK-002" {
				t.Error("TASK-002 should have been deleted from archives")
			}
		}
		if len(newTasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(newTasks))
		}
	})

	t.Run("prefers_tasks_over_archived_when_same_id_exists", func(t *testing.T) {
		tasks := []Task{
			{ID: "TASK-001", Title: "Active Task"},
		}
		archived := []Task{
			{ID: "TASK-001", Title: "Archived Task"},
		}

		newTasks, newArchived, deleted := DeleteTaskByID("TASK-001", tasks, archived)

		if !deleted {
			t.Error("expected deleted to be true")
		}
		if len(newTasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(newTasks))
		}
		if len(newArchived) != 1 {
			t.Errorf("expected 1 archived task (unchanged), got %d", len(newArchived))
		}
	})

	t.Run("returns_false_when_task_not_found", func(t *testing.T) {
		tasks := []Task{
			{ID: "TASK-001", Title: "Task 1"},
		}
		archived := []Task{
			{ID: "TASK-002", Title: "Archived 1"},
		}

		newTasks, newArchived, deleted := DeleteTaskByID("TASK-999", tasks, archived)

		if deleted {
			t.Error("expected deleted to be false")
		}
		if len(newTasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(newTasks))
		}
		if len(newArchived) != 1 {
			t.Errorf("expected 1 archived task, got %d", len(newArchived))
		}
	})

	t.Run("deletes_first_task_in_list", func(t *testing.T) {
		tasks := []Task{
			{ID: "TASK-001", Title: "Task 1"},
			{ID: "TASK-002", Title: "Task 2"},
		}
		archived := []Task{}

		newTasks, _, deleted := DeleteTaskByID("TASK-001", tasks, archived)

		if !deleted {
			t.Error("expected deleted to be true")
		}
		if len(newTasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(newTasks))
		}
		if newTasks[0].ID != "TASK-002" {
			t.Errorf("expected TASK-002 to remain, got %s", newTasks[0].ID)
		}
	})

	t.Run("deletes_last_task_in_list", func(t *testing.T) {
		tasks := []Task{
			{ID: "TASK-001", Title: "Task 1"},
			{ID: "TASK-002", Title: "Task 2"},
		}
		archived := []Task{}

		newTasks, _, deleted := DeleteTaskByID("TASK-002", tasks, archived)

		if !deleted {
			t.Error("expected deleted to be true")
		}
		if len(newTasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(newTasks))
		}
		if newTasks[0].ID != "TASK-001" {
			t.Errorf("expected TASK-001 to remain, got %s", newTasks[0].ID)
		}
	})

	t.Run("deletes_only_task_in_archived_list", func(t *testing.T) {
		tasks := []Task{}
		archived := []Task{
			{ID: "TASK-001", Title: "Only Archived"},
		}

		_, newArchived, deleted := DeleteTaskByID("TASK-001", tasks, archived)

		if !deleted {
			t.Error("expected deleted to be true")
		}
		if len(newArchived) != 0 {
			t.Errorf("expected 0 archived tasks, got %d", len(newArchived))
		}
	})
}
