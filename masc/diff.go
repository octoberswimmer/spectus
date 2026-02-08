//go:build js && wasm

package main

import (
	"fmt"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

func buildCommitDiff(repo RepoSelection, oldKanban, oldArchive, newKanban, newArchive string) string {
	parts := []string{}
	if diff := diffFile(repo.KanbanPath, oldKanban, newKanban); diff != "" {
		parts = append(parts, diff)
	}
	if diff := diffFile(repo.ArchivePath, oldArchive, newArchive); diff != "" {
		parts = append(parts, diff)
	}
	return strings.Join(parts, "\n\n")
}

func diffFile(path, oldContent, newContent string) string {
	if oldContent == newContent {
		return ""
	}
	edits := myers.ComputeEdits(span.URIFromPath(path), oldContent, newContent)
	if len(edits) == 0 {
		return ""
	}
	diff := gotextdiff.ToUnified("a/"+path, "b/"+path, oldContent, edits)
	return strings.TrimRight(fmt.Sprint(diff), "\n")
}
