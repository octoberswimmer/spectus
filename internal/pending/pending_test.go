package pending

import (
	"testing"
)

type mockStorage struct {
	data map[string]string
}

func newMockStorage() *mockStorage {
	return &mockStorage{data: make(map[string]string)}
}

func (m *mockStorage) GetItem(key string) (string, bool) {
	val, ok := m.data[key]
	return val, ok
}

func (m *mockStorage) SetItem(key, value string) {
	m.data[key] = value
}

func (m *mockStorage) RemoveItem(key string) {
	delete(m.data, key)
}

func TestSaveAndLoad(t *testing.T) {
	storage := newMockStorage()

	err := Save(storage, "owner/repo", "# Kanban\n", "# Archive\n", "Fix bug", "# Base Kanban\n", "# Base Archive\n")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(storage, "owner/repo")
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", loaded.Repo, "owner/repo")
	}
	if loaded.KanbanMarkdown != "# Kanban\n" {
		t.Errorf("KanbanMarkdown = %q, want %q", loaded.KanbanMarkdown, "# Kanban\n")
	}
	if loaded.ArchiveMarkdown != "# Archive\n" {
		t.Errorf("ArchiveMarkdown = %q, want %q", loaded.ArchiveMarkdown, "# Archive\n")
	}
	if loaded.CommitMessage != "Fix bug" {
		t.Errorf("CommitMessage = %q, want %q", loaded.CommitMessage, "Fix bug")
	}
	if loaded.BaseKanbanMarkdown != "# Base Kanban\n" {
		t.Errorf("BaseKanbanMarkdown = %q, want %q", loaded.BaseKanbanMarkdown, "# Base Kanban\n")
	}
	if loaded.BaseArchiveMarkdown != "# Base Archive\n" {
		t.Errorf("BaseArchiveMarkdown = %q, want %q", loaded.BaseArchiveMarkdown, "# Base Archive\n")
	}
}

func TestLoadEmpty(t *testing.T) {
	storage := newMockStorage()

	loaded := Load(storage, "owner/repo")
	if loaded != nil {
		t.Errorf("Load on empty storage returned %v, want nil", loaded)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	storage := newMockStorage()
	storage.SetItem(storageKey("owner/repo"), "not valid json")

	loaded := Load(storage, "owner/repo")
	if loaded != nil {
		t.Errorf("Load with invalid JSON returned %v, want nil", loaded)
	}
}

func TestClear(t *testing.T) {
	storage := newMockStorage()

	Save(storage, "owner/repo", "# Kanban\n", "# Archive\n", "", "", "")
	Clear(storage, "owner/repo")

	loaded := Load(storage, "owner/repo")
	if loaded != nil {
		t.Errorf("Load after Clear returned %v, want nil", loaded)
	}
}

func TestHasPending(t *testing.T) {
	tests := []struct {
		name  string
		saved *Changes
		want  bool
	}{
		{
			name:  "nil_saved",
			saved: nil,
			want:  false,
		},
		{
			name:  "non_nil_saved",
			saved: &Changes{Repo: "owner/repo"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasPending(tt.saved)
			if got != tt.want {
				t.Errorf("HasPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSavePreservesComplexMarkdown(t *testing.T) {
	storage := newMockStorage()

	kanban := `# Board Config
columns: To Do, In Progress, Done
categories: Bug, Feature
users: @alice, @bob

---

## To Do

### [T001] Fix login bug
- status: To Do
- category: Bug
- assignees: @alice
- created: 2024-01-15
- [ ] Investigate issue (due 2024-01-20)
- [x] Write test case

Some description here.
`

	archive := `# Archive

## Completed Tasks

### [T000] Setup project
- status: Done
- completed: 2024-01-10
`

	err := Save(storage, "myorg/myrepo", kanban, archive, "", "", "")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(storage, "myorg/myrepo")
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.KanbanMarkdown != kanban {
		t.Errorf("KanbanMarkdown not preserved correctly:\ngot:\n%s\nwant:\n%s", loaded.KanbanMarkdown, kanban)
	}
	if loaded.ArchiveMarkdown != archive {
		t.Errorf("ArchiveMarkdown not preserved correctly:\ngot:\n%s\nwant:\n%s", loaded.ArchiveMarkdown, archive)
	}
}

func TestPerRepoStorage(t *testing.T) {
	storage := newMockStorage()

	Save(storage, "owner/repo-a", "# Kanban A\n", "# Archive A\n", "", "", "")
	Save(storage, "owner/repo-b", "# Kanban B\n", "# Archive B\n", "", "", "")

	loadedA := Load(storage, "owner/repo-a")
	if loadedA == nil {
		t.Fatal("Load for repo-a returned nil")
	}
	if loadedA.KanbanMarkdown != "# Kanban A\n" {
		t.Errorf("KanbanMarkdown for repo-a = %q, want %q", loadedA.KanbanMarkdown, "# Kanban A\n")
	}

	loadedB := Load(storage, "owner/repo-b")
	if loadedB == nil {
		t.Fatal("Load for repo-b returned nil")
	}
	if loadedB.KanbanMarkdown != "# Kanban B\n" {
		t.Errorf("KanbanMarkdown for repo-b = %q, want %q", loadedB.KanbanMarkdown, "# Kanban B\n")
	}

	Clear(storage, "owner/repo-a")

	if Load(storage, "owner/repo-a") != nil {
		t.Error("repo-a should be cleared")
	}
	if Load(storage, "owner/repo-b") == nil {
		t.Error("repo-b should still exist")
	}
}

func TestCaseInsensitiveRepoKey(t *testing.T) {
	storage := newMockStorage()

	Save(storage, "Owner/Repo", "# Kanban\n", "# Archive\n", "", "", "")

	loaded := Load(storage, "owner/repo")
	if loaded == nil {
		t.Fatal("Load with different case returned nil")
	}
	if loaded.KanbanMarkdown != "# Kanban\n" {
		t.Errorf("KanbanMarkdown = %q, want %q", loaded.KanbanMarkdown, "# Kanban\n")
	}

	Clear(storage, "OWNER/REPO")

	if Load(storage, "owner/repo") != nil {
		t.Error("Clear with different case should have cleared the data")
	}
}

func TestSaveAndLoadWithBaseVersions(t *testing.T) {
	storage := newMockStorage()

	err := Save(storage, "owner/repo", "# Local\n", "# Local Archive\n", "msg", "# Base\n", "# Base Archive\n")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(storage, "owner/repo")
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.KanbanMarkdown != "# Local\n" {
		t.Errorf("KanbanMarkdown = %q, want %q", loaded.KanbanMarkdown, "# Local\n")
	}
	if loaded.ArchiveMarkdown != "# Local Archive\n" {
		t.Errorf("ArchiveMarkdown = %q, want %q", loaded.ArchiveMarkdown, "# Local Archive\n")
	}
	if loaded.BaseKanbanMarkdown != "# Base\n" {
		t.Errorf("BaseKanbanMarkdown = %q, want %q", loaded.BaseKanbanMarkdown, "# Base\n")
	}
	if loaded.BaseArchiveMarkdown != "# Base Archive\n" {
		t.Errorf("BaseArchiveMarkdown = %q, want %q", loaded.BaseArchiveMarkdown, "# Base Archive\n")
	}
}

func TestBaseVersionsForMergeDetection(t *testing.T) {
	t.Run("detect_remote_unchanged_when_base_equals_remote", func(t *testing.T) {
		storage := newMockStorage()

		baseKanban := "# Kanban\n## To Do\n"
		baseArchive := "# Archive\n"
		localKanban := "# Kanban\n## To Do\n### [T001] New Task\n"
		localArchive := "# Archive\n"

		Save(storage, "owner/repo", localKanban, localArchive, "Add task", baseKanban, baseArchive)

		loaded := Load(storage, "owner/repo")
		if loaded == nil {
			t.Fatal("Load returned nil")
		}

		// Simulate fetching from remote - same as base (no changes)
		remoteKanban := baseKanban
		remoteArchive := baseArchive

		// Check if merge is needed
		kanbanChanged := loaded.BaseKanbanMarkdown != "" && loaded.BaseKanbanMarkdown != remoteKanban
		archiveChanged := loaded.BaseArchiveMarkdown != "" && loaded.BaseArchiveMarkdown != remoteArchive

		if kanbanChanged || archiveChanged {
			t.Error("should not detect changes when remote equals base")
		}
	})

	t.Run("detect_remote_changed_when_base_differs_from_remote", func(t *testing.T) {
		storage := newMockStorage()

		baseKanban := "# Kanban\n## To Do\n"
		baseArchive := "# Archive\n"
		localKanban := "# Kanban\n## To Do\n### [T001] My Task\n"
		localArchive := "# Archive\n"

		Save(storage, "owner/repo", localKanban, localArchive, "Add task", baseKanban, baseArchive)

		loaded := Load(storage, "owner/repo")
		if loaded == nil {
			t.Fatal("Load returned nil")
		}

		// Simulate fetching from remote - different from base (collaborator added a task)
		remoteKanban := "# Kanban\n## To Do\n### [T002] Collaborator Task\n"
		remoteArchive := "# Archive\n"

		// Check if merge is needed
		kanbanChanged := loaded.BaseKanbanMarkdown != "" && loaded.BaseKanbanMarkdown != remoteKanban
		archiveChanged := loaded.BaseArchiveMarkdown != "" && loaded.BaseArchiveMarkdown != remoteArchive

		if !kanbanChanged {
			t.Error("should detect kanban changes when remote differs from base")
		}
		if archiveChanged {
			t.Error("should not detect archive changes when remote equals base")
		}
	})

	t.Run("no_merge_needed_when_base_is_empty", func(t *testing.T) {
		storage := newMockStorage()

		// Saved without base versions (backward compatibility)
		localKanban := "# Kanban\n"
		localArchive := "# Archive\n"

		Save(storage, "owner/repo", localKanban, localArchive, "", "", "")

		loaded := Load(storage, "owner/repo")
		if loaded == nil {
			t.Fatal("Load returned nil")
		}

		// Even if remote is different, we can't merge without a base
		remoteKanban := "# Different Kanban\n"

		// Check logic - empty base means no merge possible
		kanbanChanged := loaded.BaseKanbanMarkdown != "" && loaded.BaseKanbanMarkdown != remoteKanban

		if kanbanChanged {
			t.Error("should not detect changes when base is empty (no merge possible)")
		}
	})

	t.Run("archive_changes_detected_independently", func(t *testing.T) {
		storage := newMockStorage()

		baseKanban := "# Kanban\n"
		baseArchive := "# Archive\n## Completed\n"
		localKanban := "# Kanban\n"
		localArchive := "# Archive\n## Completed\n### [T001] Archived Task\n"

		Save(storage, "owner/repo", localKanban, localArchive, "Archive task", baseKanban, baseArchive)

		loaded := Load(storage, "owner/repo")
		if loaded == nil {
			t.Fatal("Load returned nil")
		}

		// Remote: kanban unchanged, archive changed
		remoteKanban := baseKanban
		remoteArchive := "# Archive\n## Completed\n### [T002] Other Archived Task\n"

		kanbanChanged := loaded.BaseKanbanMarkdown != "" && loaded.BaseKanbanMarkdown != remoteKanban
		archiveChanged := loaded.BaseArchiveMarkdown != "" && loaded.BaseArchiveMarkdown != remoteArchive

		if kanbanChanged {
			t.Error("should not detect kanban changes when remote kanban equals base")
		}
		if !archiveChanged {
			t.Error("should detect archive changes when remote archive differs from base")
		}
	})
}

func TestBackwardCompatibilityWithoutBaseVersions(t *testing.T) {
	storage := newMockStorage()

	// Manually create old format data without base versions
	oldFormatData := `{"repo":"owner/repo","kanban":"# Kanban\n","archive":"# Archive\n","commitMessage":"msg"}`
	storage.SetItem(storageKey("owner/repo"), oldFormatData)

	loaded := Load(storage, "owner/repo")
	if loaded == nil {
		t.Fatal("Load returned nil for old format")
	}

	if loaded.KanbanMarkdown != "# Kanban\n" {
		t.Errorf("KanbanMarkdown not loaded correctly: %q", loaded.KanbanMarkdown)
	}
	if loaded.BaseKanbanMarkdown != "" {
		t.Errorf("BaseKanbanMarkdown should be empty for old format, got %q", loaded.BaseKanbanMarkdown)
	}
	if loaded.BaseArchiveMarkdown != "" {
		t.Errorf("BaseArchiveMarkdown should be empty for old format, got %q", loaded.BaseArchiveMarkdown)
	}
}

func TestPendingChangesPreservedAcrossRepoSwitch(t *testing.T) {
	storage := newMockStorage()

	Save(storage, "owner/repo-a", "# Kanban A\n", "# Archive A\n", "", "", "")

	savedA := Load(storage, "owner/repo-a")
	if savedA == nil {
		t.Fatal("Load for repo-a returned nil after saving")
	}

	savedB := Load(storage, "owner/repo-b")
	if savedB != nil {
		t.Error("Load for repo-b should return nil (no pending changes)")
	}

	savedAStillExists := Load(storage, "owner/repo-a")
	if savedAStillExists == nil {
		t.Fatal("Pending changes for repo-a should still exist after checking repo-b")
	}
	if savedAStillExists.KanbanMarkdown != "# Kanban A\n" {
		t.Errorf("KanbanMarkdown = %q, want %q", savedAStillExists.KanbanMarkdown, "# Kanban A\n")
	}
}
