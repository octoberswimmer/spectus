package main

// todoItem is a single incomplete subtask flattened out of its parent task,
// used by the TODO modal and the due panel.
type todoItem struct {
	TaskID          string
	TaskTitle       string
	TaskAssignees   []string
	SubtaskIndex    int
	SubtaskID       string
	SubtaskText     string
	SubtaskDueDate  string
	SubtaskSortDate string
}

// todoGroup collects every todoItem belonging to a single task.
type todoGroup struct {
	TaskID string
	Items  []todoItem
}

// groupTodoItemsByTask groups items by TaskID while preserving the order in
// which each task is first seen. Because todo items are sorted by due date,
// subtasks of the same task are not necessarily contiguous; grouping here
// guarantees each task produces exactly one group so rendered element keys stay
// unique.
func groupTodoItemsByTask(items []todoItem) []todoGroup {
	order := make([]string, 0, len(items))
	byTask := make(map[string]*todoGroup, len(items))
	for _, item := range items {
		group, ok := byTask[item.TaskID]
		if !ok {
			order = append(order, item.TaskID)
			group = &todoGroup{TaskID: item.TaskID}
			byTask[item.TaskID] = group
		}
		group.Items = append(group.Items, item)
	}
	groups := make([]todoGroup, 0, len(order))
	for _, id := range order {
		groups = append(groups, *byTask[id])
	}
	return groups
}
