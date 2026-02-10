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

	err := Save(storage, "owner/repo", "# Kanban\n", "# Archive\n")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(storage)
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
}

func TestLoadEmpty(t *testing.T) {
	storage := newMockStorage()

	loaded := Load(storage)
	if loaded != nil {
		t.Errorf("Load on empty storage returned %v, want nil", loaded)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	storage := newMockStorage()
	storage.SetItem(StorageKey, "not valid json")

	loaded := Load(storage)
	if loaded != nil {
		t.Errorf("Load with invalid JSON returned %v, want nil", loaded)
	}
}

func TestClear(t *testing.T) {
	storage := newMockStorage()

	Save(storage, "owner/repo", "# Kanban\n", "# Archive\n")
	Clear(storage)

	loaded := Load(storage)
	if loaded != nil {
		t.Errorf("Load after Clear returned %v, want nil", loaded)
	}
}

func TestShouldRestore(t *testing.T) {
	tests := []struct {
		name       string
		saved      *Changes
		loadedRepo string
		want       bool
	}{
		{
			name:       "nil saved",
			saved:      nil,
			loadedRepo: "owner/repo",
			want:       false,
		},
		{
			name:       "matching repo",
			saved:      &Changes{Repo: "owner/repo"},
			loadedRepo: "owner/repo",
			want:       true,
		},
		{
			name:       "matching repo case insensitive",
			saved:      &Changes{Repo: "Owner/Repo"},
			loadedRepo: "owner/repo",
			want:       true,
		},
		{
			name:       "different repo",
			saved:      &Changes{Repo: "owner/other"},
			loadedRepo: "owner/repo",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldRestore(tt.saved, tt.loadedRepo)
			if got != tt.want {
				t.Errorf("ShouldRestore() = %v, want %v", got, tt.want)
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

	err := Save(storage, "myorg/myrepo", kanban, archive)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(storage)
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
