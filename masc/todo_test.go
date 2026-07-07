package main

import (
	"testing"
)

// non_contiguous_same_task_items_collapse_into_one_group verifies the fix for
// the "duplicate sibling key" render panic: buildTodoItems sorts by due date,
// so subtasks of the same task can be interleaved with other tasks. Grouping
// must still yield a single group per task.
func TestGroupTodoItemsByTask_non_contiguous_same_task_items_collapse_into_one_group(t *testing.T) {
	items := []todoItem{
		{TaskID: "TASK-A", SubtaskID: "ST-1", SubtaskDueDate: "2026-01-01"},
		{TaskID: "TASK-B", SubtaskID: "ST-2"},
		{TaskID: "TASK-A", SubtaskID: "ST-3"},
	}

	groups := groupTodoItemsByTask(items)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].TaskID != "TASK-A" || groups[1].TaskID != "TASK-B" {
		t.Fatalf("expected first-seen order [TASK-A TASK-B], got [%s %s]", groups[0].TaskID, groups[1].TaskID)
	}
	if len(groups[0].Items) != 2 {
		t.Fatalf("expected TASK-A to collect both of its subtasks, got %d", len(groups[0].Items))
	}
	if len(groups[1].Items) != 1 {
		t.Fatalf("expected TASK-B to have 1 subtask, got %d", len(groups[1].Items))
	}
}

// group_keys_are_unique guards the invariant the render code relies on: every
// group's TaskID key is distinct, so masc never sees duplicate sibling keys.
func TestGroupTodoItemsByTask_group_keys_are_unique(t *testing.T) {
	items := []todoItem{
		{TaskID: "TASK-A", SubtaskID: "ST-1"},
		{TaskID: "TASK-B", SubtaskID: "ST-2"},
		{TaskID: "TASK-A", SubtaskID: "ST-3"},
		{TaskID: "TASK-C", SubtaskID: "ST-4"},
		{TaskID: "TASK-B", SubtaskID: "ST-5"},
	}

	groups := groupTodoItemsByTask(items)

	seen := map[string]bool{}
	for _, group := range groups {
		if seen[group.TaskID] {
			t.Fatalf("duplicate group key %q", group.TaskID)
		}
		seen[group.TaskID] = true
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3 unique groups, got %d", len(groups))
	}
}

func TestGroupTodoItemsByTask_empty_input_yields_no_groups(t *testing.T) {
	if groups := groupTodoItemsByTask(nil); len(groups) != 0 {
		t.Fatalf("expected no groups for empty input, got %d", len(groups))
	}
}
