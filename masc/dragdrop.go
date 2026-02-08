//go:build js && wasm

package main

import (
	"github.com/octoberswimmer/masc"
	"github.com/octoberswimmer/masc/event"
)

type DragDropState struct {
	DraggingTaskID string
	OverTaskID     string
	OverColumnID   string
}

func (d *DragDropState) Reset() {
	d.DraggingTaskID = ""
	d.OverTaskID = ""
	d.OverColumnID = ""
}

func (d DragDropState) TaskCardMarkups(taskID, columnID string, send func(masc.Msg)) []masc.MarkupOrChild {
	classes := []string{"task-card"}
	if d.DraggingTaskID == taskID {
		classes = append(classes, "dragging")
	}
	if d.OverTaskID == taskID {
		classes = append(classes, "drag-over")
	}

	return []masc.MarkupOrChild{
		masc.Markup(
			masc.Class(classes...),
			masc.Attribute("draggable", "true"),
			masc.Attribute("data-task-id", taskID),
			event.DragStart(func(e *masc.Event) {
				send(DragStartTask{TaskID: taskID})
				dataTransfer := e.Get("dataTransfer")
				if dataTransfer.Truthy() {
					dataTransfer.Call("setData", "text/plain", taskID)
					dataTransfer.Set("effectAllowed", "move")
				}
			}),
			event.DragEnd(func(e *masc.Event) { send(DragEndTask{}) }),
			event.DragOver(func(e *masc.Event) {
				send(DragOverTask{TaskID: taskID, ColumnID: columnID})
				dataTransfer := e.Get("dataTransfer")
				if dataTransfer.Truthy() {
					dataTransfer.Set("dropEffect", "move")
				}
			}).PreventDefault().StopPropagation(),
			event.Drop(func(e *masc.Event) {
				send(DropOnTask{TargetTaskID: taskID, ColumnID: columnID})
			}).PreventDefault().StopPropagation(),
		),
	}
}

func (d DragDropState) ColumnListMarkups(columnID string, empty bool, send func(masc.Msg)) []masc.MarkupOrChild {
	classes := []string{"task-list"}
	if empty && d.OverColumnID == columnID && d.OverTaskID == "" {
		classes = append(classes, "drag-over-empty")
	}

	return []masc.MarkupOrChild{
		masc.Markup(
			masc.Class(classes...),
			masc.Attribute("data-column-id", columnID),
			event.DragOver(func(e *masc.Event) {
				send(DragOverColumn{ColumnID: columnID})
				dataTransfer := e.Get("dataTransfer")
				if dataTransfer.Truthy() {
					dataTransfer.Set("dropEffect", "move")
				}
			}).PreventDefault(),
			event.Drop(func(e *masc.Event) {
				send(DropOnColumn{ColumnID: columnID})
			}).PreventDefault(),
		),
	}
}
