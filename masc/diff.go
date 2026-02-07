//go:build js && wasm

package main

import (
	"fmt"
	"strings"
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
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)
	diffLines := diffLineChanges(oldLines, newLines, 3)
	if len(diffLines) == 0 {
		return ""
	}
	header := fmt.Sprintf("--- a/%s\n+++ b/%s", path, path)
	return header + "\n" + strings.Join(diffLines, "\n")
}

func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.Split(content, "\n")
}

type diffOp struct {
	kind byte
	line string
}

type diffHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	ops      []diffOp
}

func diffLineChanges(oldLines, newLines []string, context int) []string {
	n := len(oldLines)
	m := len(newLines)
	if n == 0 && m == 0 {
		return []string{}
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	ops := make([]diffOp, 0, n+m)
	i := 0
	j := 0
	for i < n && j < m {
		if oldLines[i] == newLines[j] {
			ops = append(ops, diffOp{kind: ' ', line: oldLines[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, diffOp{kind: '-', line: oldLines[i]})
			i++
		} else {
			ops = append(ops, diffOp{kind: '+', line: newLines[j]})
			j++
		}
	}
	for i < n {
		ops = append(ops, diffOp{kind: '-', line: oldLines[i]})
		i++
	}
	for j < m {
		ops = append(ops, diffOp{kind: '+', line: newLines[j]})
		j++
	}

	hunks := buildHunks(ops, context)
	out := make([]string, 0, len(hunks)*4)
	for _, h := range hunks {
		out = append(out, fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.oldStart, h.oldCount, h.newStart, h.newCount))
		for _, op := range h.ops {
			out = append(out, string(op.kind)+op.line)
		}
	}
	return out
}

func buildHunks(ops []diffOp, context int) []diffHunk {
	if len(ops) == 0 {
		return nil
	}
	if context < 0 {
		context = 0
	}

	oldAt := make([]int, len(ops))
	newAt := make([]int, len(ops))
	oldLine := 1
	newLine := 1
	for i, op := range ops {
		oldAt[i] = oldLine
		newAt[i] = newLine
		if op.kind != '+' {
			oldLine++
		}
		if op.kind != '-' {
			newLine++
		}
	}

	hunks := []diffHunk{}
	i := 0
	for i < len(ops) {
		for i < len(ops) && ops[i].kind == ' ' {
			i++
		}
		if i >= len(ops) {
			break
		}
		start := i - context
		if start < 0 {
			start = 0
		}
		end := i
		lastChange := i
		for end < len(ops) {
			if ops[end].kind != ' ' {
				lastChange = end
			}
			if end > lastChange+context {
				break
			}
			end++
		}

		oldStart := oldAt[start]
		newStart := newAt[start]
		oldCount := 0
		newCount := 0
		for _, op := range ops[start:end] {
			if op.kind != '+' {
				oldCount++
			}
			if op.kind != '-' {
				newCount++
			}
		}

		hunks = append(hunks, diffHunk{
			oldStart: oldStart,
			oldCount: oldCount,
			newStart: newStart,
			newCount: newCount,
			ops:      ops[start:end],
		})
		i = end
	}

	return hunks
}
