//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/octoberswimmer/masc"
	"github.com/octoberswimmer/masc/elem"
	"github.com/octoberswimmer/masc/event"
	"github.com/octoberswimmer/spectus/internal/markdown"
)

var idRand = rand.New(rand.NewSource(time.Now().UnixNano()))

type TaskForm struct {
	ID          string
	Title       string
	Status      string
	Category    string
	Assignees   string
	Tags        string
	Created     string
	Completed   string
	Description string
	Notes       string
	Subtasks    []Subtask
}

type Program struct {
	masc.Core

	cfg           ClientConfig
	appInstallURL string

	token    string
	loggedIn bool
	viewer   User

	repo       RepoSelection
	branch     string
	headOID    string
	repoLoaded bool

	config              BoardConfig
	tasks               []Task
	archived            []Task
	repos               []RepoOption
	selectedRepo        string
	repoInstallRequired bool

	lastSavedKanban  string
	lastSavedArchive string

	commitMessageDraft   string
	commitDiff           string
	pendingCommitKanban  string
	pendingCommitArchive string
	tagSuggestionsOpen   bool

	loading          bool
	commitInProgress bool
	dirty            bool
	error            string
	status           string
	statusSeq        int

	filters []Filter
	search  string

	modal             ModalMode
	detailTaskID      string
	form              TaskForm
	archiveSearch     string
	newSubtaskText    string
	newSubtaskDue     string
	detailSubtaskText string
	detailSubtaskDue  string

	dragDrop DragDropState
}

func NewProgram(cfg ClientConfig) *Program {
	return &Program{
		cfg:           cfg,
		appInstallURL: strings.TrimSpace(cfg.AppInstallURL),
	}
}

func (p *Program) Init() masc.Cmd {
	return nil
}

func (p *Program) Update(msg masc.Msg) (masc.Model, masc.Cmd) {
	switch msg := msg.(type) {
	case SetSession:
		return p.handleSession(msg)
	case ViewerLoaded:
		p.viewer = msg.Viewer
		p.loading = true
		p.error = ""
		statusCmd := p.setStatus("Loading repositories…", false)
		return p, batchCmds(statusCmd, p.fetchReposCmd())
	case ReposLoaded:
		if msg.Unauthorized {
			return p.handleUnauthorized()
		}
		p.loading = false
		if msg.Error != "" {
			p.error = msg.Error
			return p, nil
		}
		p.repos = msg.Repos
		p.repoInstallRequired = len(p.repos) == 0
		if p.repoInstallRequired {
			p.error = ""
			p.setStatus("", false)
			return p, nil
		}
		selected := p.resolveSelectedRepo()
		if selected == "" {
			p.error = "No repositories available for this GitHub App installation."
			return p, nil
		}
		p.repoInstallRequired = false
		p.selectedRepo = selected
		p.loading = true
		statusCmd := p.setStatus("Loading repository…", false)
		return p, batchCmds(statusCmd, p.saveRepoSelectionCmd(selected), p.loadRepoCmd())
	case LoadError:
		if msg.Unauthorized {
			return p.handleUnauthorized()
		}
		p.loading = false
		p.commitInProgress = false
		p.error = msg.Error
		return p, nil
	case LoadRepo:
		if p.loading {
			return p, nil
		}
		if strings.TrimSpace(p.repoToLoad()) == "" {
			p.error = "No repository selected."
			return p, nil
		}
		p.error = ""
		statusCmd := p.setStatus("Loading repository…", false)
		p.loading = true
		return p, batchCmds(statusCmd, p.loadRepoCmd())
	case RepoLoaded:
		p.loading = false
		p.commitInProgress = false
		p.error = ""
		statusCmd := p.setStatus("Repository loaded.", true)
		p.repo = msg.Repo
		p.selectedRepo = msg.Repo.Repo
		p.branch = msg.Branch
		p.headOID = msg.HeadOID
		p.repoLoaded = true

		kanbanContent := msg.KanbanContent
		if msg.MissingKanban || strings.TrimSpace(kanbanContent) == "" {
			kanbanContent = createDefaultKanbanContent()
		}
		config, tasks := parseKanban(kanbanContent)
		p.config = config
		p.tasks = tasks
		p.lastSavedKanban = msg.KanbanContent

		archiveContent := msg.ArchiveContent
		if msg.MissingArchive || strings.TrimSpace(archiveContent) == "" {
			archiveContent = createDefaultArchiveContent()
		}
		p.archived = parseArchive(archiveContent)
		p.lastSavedArchive = msg.ArchiveContent

		p.dirty = msg.MissingKanban || msg.MissingArchive
		return p, statusCmd
	case CommitChanges:
		if !p.repoLoaded || p.commitInProgress || !p.hasPendingCommitChanges() {
			return p, nil
		}
		p.modal = ModalCommit
		p.error = ""
		p.commitMessageDraft = ""
		p.commitDiff = buildCommitDiff(p.repo, p.lastSavedKanban, p.lastSavedArchive, p.generateKanbanMarkdown(), p.generateArchiveMarkdown())
		return p, nil
	case CommitResult:
		if msg.Unauthorized {
			return p.handleUnauthorized()
		}
		p.commitInProgress = false
		if msg.Error != "" {
			p.error = msg.Error
			return p, nil
		}
		p.dirty = false
		if p.pendingCommitKanban != "" || p.pendingCommitArchive != "" {
			p.lastSavedKanban = p.pendingCommitKanban
			p.lastSavedArchive = p.pendingCommitArchive
			p.pendingCommitKanban = ""
			p.pendingCommitArchive = ""
		}
		if msg.OID != "" {
			p.headOID = msg.OID
		}
		if msg.URL != "" {
			return p, p.setStatus("Committed: "+msg.URL, true)
		} else {
			return p, p.setStatus("Commit saved.", true)
		}
	case ClearStatus:
		if msg.Seq == p.statusSeq {
			p.status = ""
		}
		return p, nil
	case UpdateSearch:
		p.search = msg.Value
		return p, nil
	case AddFilter:
		value := strings.TrimSpace(msg.Value)
		if value == "" {
			return p, nil
		}
		for _, f := range p.filters {
			if f.Type == msg.Type && f.Value == value {
				return p, nil
			}
		}
		p.filters = append(p.filters, Filter{Type: msg.Type, Value: value})
		return p, nil
	case RemoveFilter:
		if msg.Index >= 0 && msg.Index < len(p.filters) {
			p.filters = append(p.filters[:msg.Index], p.filters[msg.Index+1:]...)
		}
		return p, nil
	case ClearFilters:
		p.filters = nil
		return p, nil
	case OpenModal:
		p.modal = msg.Mode
		p.error = ""
		switch msg.Mode {
		case ModalDetail:
			p.detailTaskID = msg.TaskID
			p.detailSubtaskText = ""
			p.detailSubtaskDue = ""
		case ModalEdit:
			p.detailTaskID = ""
			p.form = p.formForTask(msg.TaskID)
			p.newSubtaskText = ""
			p.newSubtaskDue = ""
			p.tagSuggestionsOpen = false
		case ModalArchive:
			p.archiveSearch = ""
		case ModalCommit:
			p.commitMessageDraft = ""
			p.commitDiff = buildCommitDiff(p.repo, p.lastSavedKanban, p.lastSavedArchive, p.generateKanbanMarkdown(), p.generateArchiveMarkdown())
		}
		return p, nil
	case OpenDetailFromTodo:
		p.modal = ModalNone
		p.detailTaskID = ""
		p.newSubtaskText = ""
		p.newSubtaskDue = ""
		p.detailSubtaskText = ""
		p.detailSubtaskDue = ""
		p.tagSuggestionsOpen = false
		return p, func() masc.Msg {
			return OpenModal{Mode: ModalDetail, TaskID: msg.TaskID}
		}
	case CloseModal:
		p.modal = ModalNone
		p.detailTaskID = ""
		p.newSubtaskText = ""
		p.newSubtaskDue = ""
		p.detailSubtaskText = ""
		p.detailSubtaskDue = ""
		p.tagSuggestionsOpen = false
		return p, nil
	case UpdateFormField:
		switch msg.Field {
		case "title":
			p.form.Title = msg.Value
		case "status":
			p.form.Status = msg.Value
		case "category":
			p.form.Category = msg.Value
		case "assignees":
			p.form.Assignees = msg.Value
		case "tags":
			p.form.Tags = msg.Value
			p.tagSuggestionsOpen = true
		case "created":
			p.form.Created = msg.Value
		case "completed":
			p.form.Completed = msg.Value
		case "description":
			p.form.Description = msg.Value
		case "notes":
			p.form.Notes = msg.Value
		case "new_subtask_text":
			p.newSubtaskText = msg.Value
		case "new_subtask_due":
			p.newSubtaskDue = msg.Value
		}
		return p, nil
	case UpdateCommitMessage:
		p.commitMessageDraft = msg.Value
		return p, nil
	case SelectRepo:
		selected := strings.TrimSpace(msg.FullName)
		if selected == "" || selected == p.selectedRepo {
			return p, nil
		}
		p.selectedRepo = selected
		p.loading = true
		p.error = ""
		statusCmd := p.setStatus("Loading repository…", false)
		p.repoLoaded = false
		return p, batchCmds(statusCmd, p.saveRepoSelectionCmd(selected), p.loadRepoCmd())
	case RepoSelectionSaved:
		if msg.Error != "" {
			p.error = msg.Error
		}
		return p, nil
	case OpenInstallURL:
		if strings.TrimSpace(p.appInstallURL) == "" {
			return p, nil
		}
		url := p.appInstallURL
		return p, func() masc.Msg {
			js.Global().Call("open", url, "_blank")
			return nil
		}
	case SetTagSuggestionsOpen:
		p.tagSuggestionsOpen = msg.Open
		return p, nil
	case SelectTagSuggestion:
		p.form.Tags = applyTagSuggestion(p.form.Tags, msg.Tag)
		p.tagSuggestionsOpen = false
		return p, nil
	case ConfirmCommit:
		if !p.repoLoaded || p.commitInProgress || !p.hasPendingCommitChanges() {
			return p, nil
		}
		if strings.TrimSpace(msg.Message) == "" {
			p.error = "Commit message is required."
			return p, nil
		}
		p.commitInProgress = true
		p.error = ""
		statusCmd := p.setStatus("Committing changes…", false)
		p.modal = ModalNone
		p.pendingCommitKanban = p.generateKanbanMarkdown()
		p.pendingCommitArchive = p.generateArchiveMarkdown()
		files := map[string]string{
			p.repo.KanbanPath:  p.pendingCommitKanban,
			p.repo.ArchivePath: p.pendingCommitArchive,
		}
		return p, batchCmds(statusCmd, p.commitCmd(strings.TrimSpace(msg.Message), files))
	case UpdateDetailSubtaskField:
		switch msg.Field {
		case "text":
			p.detailSubtaskText = msg.Value
		case "due":
			p.detailSubtaskDue = msg.Value
		}
		return p, nil
	case AddFormSubtask:
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			return p, nil
		}
		p.form.Subtasks = append(p.form.Subtasks, Subtask{Text: text, DueDate: normalizeDueDate(msg.DueDate)})
		p.newSubtaskText = ""
		p.newSubtaskDue = ""
		return p, nil
	case ToggleFormSubtask:
		if msg.Index >= 0 && msg.Index < len(p.form.Subtasks) {
			p.form.Subtasks[msg.Index].Completed = !p.form.Subtasks[msg.Index].Completed
		}
		return p, nil
	case UpdateFormSubtaskText:
		if msg.Index >= 0 && msg.Index < len(p.form.Subtasks) {
			p.form.Subtasks[msg.Index].Text = msg.Value
		}
		return p, nil
	case UpdateFormSubtaskDueDate:
		if msg.Index >= 0 && msg.Index < len(p.form.Subtasks) {
			p.form.Subtasks[msg.Index].DueDate = normalizeDueDate(msg.Value)
		}
		return p, nil
	case DeleteFormSubtask:
		if msg.Index >= 0 && msg.Index < len(p.form.Subtasks) {
			p.form.Subtasks = append(p.form.Subtasks[:msg.Index], p.form.Subtasks[msg.Index+1:]...)
		}
		return p, nil
	case AddTaskSubtask:
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			return p, nil
		}
		p.updateTaskSubtasks(msg.TaskID, func(task *Task) {
			task.Subtasks = append(task.Subtasks, Subtask{Text: text, DueDate: normalizeDueDate(msg.DueDate)})
		})
		p.detailSubtaskText = ""
		p.detailSubtaskDue = ""
		return p, nil
	case ToggleTaskSubtask:
		p.updateTaskSubtasks(msg.TaskID, func(task *Task) {
			if msg.Index >= 0 && msg.Index < len(task.Subtasks) {
				task.Subtasks[msg.Index].Completed = !task.Subtasks[msg.Index].Completed
			}
		})
		return p, nil
	case UpdateTaskSubtaskText:
		p.updateTaskSubtasks(msg.TaskID, func(task *Task) {
			if msg.Index >= 0 && msg.Index < len(task.Subtasks) {
				task.Subtasks[msg.Index].Text = msg.Value
			}
		})
		return p, nil
	case UpdateTaskSubtaskDueDate:
		p.updateTaskSubtasks(msg.TaskID, func(task *Task) {
			if msg.Index >= 0 && msg.Index < len(task.Subtasks) {
				task.Subtasks[msg.Index].DueDate = normalizeDueDate(msg.Value)
			}
		})
		return p, nil
	case DeleteTaskSubtask:
		p.updateTaskSubtasks(msg.TaskID, func(task *Task) {
			if msg.Index >= 0 && msg.Index < len(task.Subtasks) {
				task.Subtasks = append(task.Subtasks[:msg.Index], task.Subtasks[msg.Index+1:]...)
			}
		})
		return p, nil
	case SaveTask:
		return p.handleSaveTask()
	case DeleteTask:
		p.deleteTaskByID(msg.TaskID)
		return p, p.setStatus("Task deleted.", true)
	case ArchiveTask:
		p.archiveTask(msg.TaskID)
		return p, p.setStatus("Task archived.", true)
	case RestoreTask:
		p.restoreTask(msg.TaskID)
		return p, p.setStatus("Task restored.", true)
	case MoveTaskPosition:
		p.moveTaskWithinColumn(msg.TaskID, msg.Direction)
		return p, nil
	case CloneTask:
		return p.cloneTask(msg.TaskID)
	case AddColumn:
		p.addColumn()
		return p, nil
	case UpdateColumn:
		p.updateColumn(msg.Index, msg.Field, msg.Value)
		return p, nil
	case DeleteColumn:
		p.deleteColumn(msg.Index)
		return p, nil
	case MoveColumn:
		p.moveColumn(msg.Index, msg.Direction)
		return p, nil
	case UpdateArchiveSearch:
		p.archiveSearch = msg.Value
		return p, nil
	case DragStartTask:
		p.dragDrop.DraggingTaskID = msg.TaskID
		p.dragDrop.OverTaskID = ""
		p.dragDrop.OverColumnID = ""
		return p, nil
	case DragOverTask:
		if p.dragDrop.DraggingTaskID == "" || msg.TaskID == p.dragDrop.DraggingTaskID {
			return p, nil
		}
		p.dragDrop.OverTaskID = msg.TaskID
		p.dragDrop.OverColumnID = msg.ColumnID
		return p, nil
	case DragOverColumn:
		if p.dragDrop.DraggingTaskID == "" {
			return p, nil
		}
		p.dragDrop.OverTaskID = ""
		p.dragDrop.OverColumnID = msg.ColumnID
		return p, nil
	case DragEndTask:
		p.dragDrop.Reset()
		return p, nil
	case DropOnTask:
		p.handleDropOnTask(msg.TargetTaskID, msg.ColumnID)
		p.dragDrop.Reset()
		return p, nil
	case DropOnColumn:
		p.handleDropOnColumn(msg.ColumnID)
		p.dragDrop.Reset()
		return p, nil
	case Logout:
		return p, logoutCmd()
	}

	return p, nil
}

func (p *Program) Render(send func(masc.Msg)) masc.ComponentOrHTML {
	if !p.loggedIn {
		return p.renderLogin(send)
	}

	return elem.Div(
		p.renderHeader(send),
		masc.If(p.repoLoaded, p.renderFilterBar(send)),
		elem.Main(
			masc.Markup(
				masc.Class("container"),
				masc.Style("max-width", "none"),
				masc.Style("width", "100%"),
				masc.Style("margin", "2rem 0"),
				masc.Style("padding", "0"),
			),
			masc.If(!p.repoLoaded && !p.loading && p.repoInstallRequired, p.renderRepoInstallPrompt()),
			masc.If(!p.repoLoaded && !p.loading && !p.repoInstallRequired, p.renderWelcome()),
			masc.If(p.repoLoaded, p.renderBoard(send)),
			masc.If(p.error != "", p.renderError()),
		),
		p.renderLoadingOverlay(),
		p.renderNotification(),
		p.renderModal(send),
	)
}

func (p *Program) handleSession(msg SetSession) (masc.Model, masc.Cmd) {
	var session Session
	if msg.Session != "" {
		_ = json.Unmarshal([]byte(msg.Session), &session)
	}
	if session.AccessToken == "" {
		p.loggedIn = false
		p.token = ""
		p.repoLoaded = false
		p.loading = false
		p.setStatus("", false)
		return p, nil
	}
	p.loggedIn = true
	p.token = session.AccessToken
	p.selectedRepo = strings.TrimSpace(session.SelectedRepo)
	p.loading = true
	p.error = ""
	p.setStatus("", false)
	return p, p.fetchViewerCmd()
}

func (p *Program) handleUnauthorized() (masc.Model, masc.Cmd) {
	p.loggedIn = false
	p.token = ""
	p.repoLoaded = false
	p.loading = false
	p.commitInProgress = false
	p.repos = nil
	p.repoInstallRequired = false
	p.selectedRepo = ""
	p.viewer = User{}
	p.modal = ModalNone
	p.error = "GitHub session expired. Please log in again."
	p.setStatus("", false)
	return p, clearSessionCmd()
}

func (p *Program) fetchViewerCmd() masc.Cmd {
	token := p.token
	return func() masc.Msg {
		client := &GraphQLClient{Token: token}
		viewer, err := fetchViewer(client)
		if err != nil {
			return LoadError{Error: err.Error(), Unauthorized: isUnauthorized(err)}
		}
		return ViewerLoaded{Viewer: viewer}
	}
}

func (p *Program) fetchReposCmd() masc.Cmd {
	token := p.token
	return func() masc.Msg {
		repos, err := fetchAccessibleRepos(token)
		if err != nil {
			return ReposLoaded{Error: err.Error(), Unauthorized: isUnauthorized(err)}
		}
		return ReposLoaded{Repos: repos}
	}
}

func (p *Program) loadRepoCmd() masc.Cmd {
	owner, name, repoName, err := splitRepo(strings.TrimSpace(p.repoToLoad()))
	if err != nil {
		return func() masc.Msg { return LoadError{Error: err.Error()} }
	}
	kanbanPath := strings.TrimSpace(p.cfg.KanbanPath)
	if kanbanPath == "" {
		kanbanPath = "kanban.md"
	}
	archivePath := strings.TrimSpace(p.cfg.ArchivePath)
	if archivePath == "" {
		archivePath = "archive.md"
	}
	token := p.token
	return func() masc.Msg {
		client := &GraphQLClient{Token: token}
		files, err := fetchRepoFiles(client, owner, name, kanbanPath, archivePath)
		if err != nil {
			return LoadError{Error: err.Error(), Unauthorized: isUnauthorized(err)}
		}
		repo := RepoSelection{
			Owner:       owner,
			Name:        name,
			Repo:        repoName,
			KanbanPath:  kanbanPath,
			ArchivePath: archivePath,
			Branch:      files.Branch,
		}
		return RepoLoaded{
			Repo:           repo,
			Branch:         files.Branch,
			HeadOID:        files.HeadOID,
			KanbanContent:  files.KanbanContent,
			ArchiveContent: files.ArchiveContent,
			MissingKanban:  files.MissingKanban,
			MissingArchive: files.MissingArchive,
		}
	}
}

func (p *Program) repoToLoad() string {
	if strings.TrimSpace(p.selectedRepo) != "" {
		return p.selectedRepo
	}
	return strings.TrimSpace(p.cfg.DefaultRepo)
}

func (p *Program) resolveSelectedRepo() string {
	if candidate := strings.TrimSpace(p.selectedRepo); candidate != "" && p.repoInList(candidate) {
		return candidate
	}
	if candidate := strings.TrimSpace(p.cfg.DefaultRepo); candidate != "" && p.repoInList(candidate) {
		return candidate
	}
	if len(p.repos) > 0 {
		return p.repos[0].FullName
	}
	return ""
}

func (p *Program) repoInList(fullName string) bool {
	for _, repo := range p.repos {
		if repo.FullName == fullName {
			return true
		}
	}
	return false
}

func (p *Program) saveRepoSelectionCmd(repo string) masc.Cmd {
	payload := map[string]string{"selected_repo": repo}
	body, _ := json.Marshal(payload)
	return func() masc.Msg {
		headers := map[string]interface{}{
			"Content-Type": "application/json",
		}
		options := map[string]interface{}{
			"method":  "POST",
			"headers": headers,
			"body":    string(body),
		}
		respValue, err := awaitPromise(js.Global().Call("fetch", "/session", js.ValueOf(options)))
		if err != nil {
			return RepoSelectionSaved{Error: err.Error()}
		}
		if !respValue.Get("ok").Bool() {
			textValue, err := awaitPromise(respValue.Call("text"))
			if err != nil {
				return RepoSelectionSaved{Error: err.Error()}
			}
			return RepoSelectionSaved{Error: fmt.Sprintf("failed to save selection: %s", strings.TrimSpace(textValue.String()))}
		}
		return RepoSelectionSaved{}
	}
}

func (p *Program) commitCmd(message string, files map[string]string) masc.Cmd {
	repoName := p.repo.Repo
	branch := p.branch
	headOID := p.headOID
	commitMessage := strings.TrimSpace(message)
	token := p.token
	return func() masc.Msg {
		if commitMessage == "" {
			return CommitResult{Error: "commit message is required"}
		}
		client := &GraphQLClient{Token: token}
		result, err := commitRepoFiles(client, repoName, branch, headOID, commitMessage, files)
		if err != nil {
			return CommitResult{Error: err.Error(), Unauthorized: isUnauthorized(err)}
		}
		return CommitResult{URL: result.URL, OID: result.OID}
	}
}

func (p *Program) renderLogin(send func(masc.Msg)) masc.ComponentOrHTML {
	return elem.Div(
		p.renderHeader(send),
		elem.Main(
			masc.Markup(masc.Class("container")),
			elem.Div(
				masc.Markup(masc.Class("summary-card")),
				elem.Heading2(masc.Text("Connect GitHub")),
				elem.Paragraph(masc.Text("This app uses GitHub OAuth and commits markdown updates directly to your repository.")),
				masc.If(p.error != "",
					elem.Paragraph(
						masc.Markup(masc.Class("form-warning")),
						masc.Text(p.error),
					),
				),
				elem.Div(
					masc.Markup(masc.Class("actions")),
					elem.Anchor(
						masc.Markup(masc.Class("btn", "btn-primary"), masc.Attribute("href", "/login")),
						masc.Text("Login with GitHub"),
					),
				),
			),
		),
	)
}

func (p *Program) renderHeader(send func(masc.Msg)) masc.ComponentOrHTML {
	return elem.Header(
		masc.Markup(masc.Class("header")),
		elem.Div(
			masc.Markup(
				masc.Class("header-content"),
				masc.Style("max-width", "none"),
				masc.Style("width", "100%"),
				masc.Style("margin", "0"),
			),
			elem.Div(
				masc.Markup(masc.Class("brand")),
				elem.Image(masc.Markup(
					masc.Class("brand-mark"),
					masc.Attribute("src", "/static/spectus.png"),
					masc.Attribute("alt", "Spectus logo"),
				)),
			),
			elem.Div(
				masc.Markup(masc.Class("header-actions")),
				masc.If(len(p.repos) > 0, p.renderRepoSelect(send)),
				masc.If(p.repoLoaded && len(p.repos) == 0,
					elem.Div(masc.Markup(masc.Class("status-pill")), masc.Text(p.repo.Repo+"@"+p.branch)),
				),
				masc.If(p.repoLoaded,
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(OpenModal{Mode: ModalEdit}) })),
						masc.Text("➕ New"),
					),
				),
				masc.If(p.repoLoaded,
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(OpenModal{Mode: ModalArchive}) })),
						masc.Text("📦 Archives"),
					),
				),
				masc.If(p.repoLoaded,
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(OpenModal{Mode: ModalColumns}) })),
						masc.Text("⚙️ Columns"),
					),
				),
				masc.If(p.repoLoaded,
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(OpenModal{Mode: ModalTodo}) })),
						masc.Text("📋 TODO"),
					),
				),
				masc.If(p.repoLoaded,
					elem.Button(
						masc.Markup(
							masc.Class("btn", "btn-primary"),
							masc.Property("disabled", !p.hasPendingCommitChanges() || p.commitInProgress),
							event.Click(func(e *masc.Event) { send(CommitChanges{}) }),
						),
						masc.Text("💾 Commit"),
					),
				),
				masc.If(p.loggedIn,
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-ghost"), event.Click(func(e *masc.Event) {
							send(LoadRepo{})
						})),
						masc.Text("↻ Reload"),
					),
				),
				masc.If(p.loggedIn,
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(Logout{}) })),
						masc.Text("Logout"),
					),
				),
			),
		),
	)
}

func (p *Program) renderRepoSelect(send func(masc.Msg)) masc.ComponentOrHTML {
	if len(p.repos) == 0 {
		return nil
	}
	current := strings.TrimSpace(p.selectedRepo)
	if current == "" && p.repo.Repo != "" {
		current = p.repo.Repo
	}
	options := make([]masc.ComponentOrHTML, 0, len(p.repos))
	for _, repo := range p.repos {
		option := elem.Option(
			masc.Markup(
				masc.Property("value", repo.FullName),
				masc.MarkupIf(repo.FullName == current, masc.Property("selected", true)),
			),
			masc.Text(repo.FullName),
		)
		options = append(options, option)
	}
	if p.appInstallURL != "" {
		options = append(options, elem.Option(
			masc.Markup(masc.Property("value", "__configure__")),
			masc.Text("Configure repos…"),
		))
	}

	return elem.Select(
		append([]masc.MarkupOrChild{
			masc.Markup(
				masc.Class("repo-select"),
				event.Change(func(e *masc.Event) {
					value := e.Target.Get("value").String()
					if value == "__configure__" {
						send(OpenInstallURL{})
						return
					}
					send(SelectRepo{FullName: value})
				}),
			),
		}, toMarkupChildren(options)...)...,
	)
}

func (p *Program) renderWelcome() masc.ComponentOrHTML {
	return elem.Div(
		masc.Markup(masc.Class("welcome")),
		elem.Heading2(masc.Text("Repository not configured")),
		elem.Paragraph(masc.Text("Ask an administrator to set the default repository for this workspace.")),
	)
}

func (p *Program) renderRepoInstallPrompt() masc.ComponentOrHTML {
	return elem.Div(
		masc.Markup(masc.Class("summary-card")),
		elem.Heading2(masc.Text("GitHub App not installed")),
		elem.Paragraph(masc.Text("Install the GitHub App to grant access to repositories.")),
		masc.If(p.appInstallURL != "", elem.Div(
			masc.Markup(masc.Class("actions")),
			elem.Anchor(
				masc.Markup(
					masc.Class("btn", "btn-primary"),
					masc.Attribute("href", p.appInstallURL),
					masc.Attribute("target", "_blank"),
					masc.Attribute("rel", "noreferrer"),
				),
				masc.Text("Install GitHub App"),
			),
		)),
	)
}

func (p *Program) renderFilterBar(send func(masc.Msg)) masc.ComponentOrHTML {
	categories, users, tags := p.uniqueValues()

	tagOptions := make([]masc.ComponentOrHTML, 0, len(tags)+1)
	tagOptions = append(tagOptions, elem.Option(masc.Markup(masc.Property("value", "")), masc.Text("Select…")))
	for _, tag := range tags {
		tagOptions = append(tagOptions, elem.Option(masc.Markup(masc.Property("value", tag)), masc.Text(tag)))
	}

	catOptions := make([]masc.ComponentOrHTML, 0, len(categories)+1)
	catOptions = append(catOptions, elem.Option(masc.Markup(masc.Property("value", "")), masc.Text("Select…")))
	for _, cat := range categories {
		catOptions = append(catOptions, elem.Option(masc.Markup(masc.Property("value", cat)), masc.Text(cat)))
	}

	userOptions := make([]masc.ComponentOrHTML, 0, len(users)+2)
	userOptions = append(userOptions, elem.Option(masc.Markup(masc.Property("value", "")), masc.Text("Select…")))
	userOptions = append(userOptions, elem.Option(masc.Markup(masc.Property("value", "<unassigned>")), masc.Text("<Unassigned>")))
	for _, user := range users {
		userOptions = append(userOptions, elem.Option(masc.Markup(masc.Property("value", normalizeUserID(user))), masc.Text(user)))
	}

	return elem.Div(
		masc.Markup(masc.Class("filter-bar")),
		elem.Div(
			masc.Markup(
				masc.Class("filter-content"),
				masc.Style("max-width", "none"),
				masc.Style("width", "100%"),
				masc.Style("margin", "0"),
			),
			elem.Div(
				masc.Markup(masc.Class("filter-row")),
				elem.Div(
					masc.Markup(masc.Class("search-wrap")),
					elem.Input(
						masc.Markup(
							masc.Class("search-input"),
							masc.Property("type", "text"),
							masc.Property("placeholder", "Search…"),
							masc.Property("value", p.search),
							event.Input(func(e *masc.Event) { send(UpdateSearch{Value: e.Target.Get("value").String()}) }),
						),
					),
				),
			),
			elem.Div(
				masc.Markup(masc.Class("filter-row")),
				elem.Div(
					masc.Markup(masc.Class("filter-group")),
					elem.Label(masc.Text("Tags:")),
					elem.Select(
						append([]masc.MarkupOrChild{
							masc.Markup(event.Change(func(e *masc.Event) {
								send(AddFilter{Type: "tag", Value: e.Target.Get("value").String()})
							})),
						}, toMarkupChildren(tagOptions)...)...,
					),
				),
				elem.Div(
					masc.Markup(masc.Class("filter-group")),
					elem.Label(masc.Text("Category:")),
					elem.Select(
						append([]masc.MarkupOrChild{
							masc.Markup(event.Change(func(e *masc.Event) {
								send(AddFilter{Type: "category", Value: e.Target.Get("value").String()})
							})),
						}, toMarkupChildren(catOptions)...)...,
					),
				),
				elem.Div(
					masc.Markup(masc.Class("filter-group")),
					elem.Label(masc.Text("User:")),
					elem.Select(
						append([]masc.MarkupOrChild{
							masc.Markup(event.Change(func(e *masc.Event) {
								send(AddFilter{Type: "user", Value: e.Target.Get("value").String()})
							})),
						}, toMarkupChildren(userOptions)...)...,
					),
				),
				elem.Button(
					masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(ClearFilters{}) })),
					masc.Text("✕ Clear all"),
				),
			),
			elem.Div(
				append([]masc.MarkupOrChild{masc.Markup(masc.Class("active-filters"))}, toMarkupChildren(p.renderFilterPills(send))...)...,
			),
		),
	)
}

func (p *Program) renderFilterPills(send func(masc.Msg)) []masc.ComponentOrHTML {
	pills := make([]masc.ComponentOrHTML, 0, len(p.filters))
	for idx, f := range p.filters {
		display := f.Value
		if f.Type == "user" && f.Value != "<unassigned>" {
			display = p.fullUserFormat(f.Value)
		}
		pills = append(pills, elem.Div(
			masc.Markup(masc.Class("filter-pill")),
			masc.Text(display),
			elem.Button(
				masc.Markup(masc.Class("btn", "btn-ghost"), event.Click(func(e *masc.Event) { send(RemoveFilter{Index: idx}) })),
				masc.Text("✕"),
			),
		))
	}
	return pills
}

func (p *Program) renderBoard(send func(masc.Msg)) masc.ComponentOrHTML {
	columns := p.config.Columns
	if len(columns) == 0 {
		columns = defaultColumns()
	}

	columnNodes := make([]masc.ComponentOrHTML, 0, len(columns))
	for _, column := range columns {
		tasks := p.filteredTasks(column.ID)
		listMarkups := p.dragDrop.ColumnListMarkups(column.ID, len(tasks) == 0, send)
		children := make([]masc.MarkupOrChild, 0, len(tasks)+2)
		children = append(children, listMarkups...)
		if len(tasks) == 0 {
			children = append(children, elem.Div(masc.Markup(masc.Class("empty-state")), masc.Text("No tasks")))
		} else {
			for _, task := range tasks {
				children = append(children, p.renderTaskCard(task, send))
			}
		}

		columnNodes = append(columnNodes,
			elem.Div(
				masc.Markup(masc.Class("kanban-column")),
				elem.Div(
					masc.Markup(masc.Class("column-header")),
					elem.Div(masc.Markup(masc.Class("column-title")), masc.Text(column.Name)),
					elem.Div(masc.Markup(masc.Class("column-count")), masc.Text(fmt.Sprintf("%d", len(tasks)))),
				),
				elem.Div(children...),
			),
		)
	}

	return elem.Div(
		append([]masc.MarkupOrChild{masc.Markup(
			masc.Class("kanban-board"),
			masc.Style("width", "100%"),
			masc.Style("min-width", "100%"),
		)}, toMarkupChildren(columnNodes)...)...,
	)
}

func (p *Program) renderTaskCard(task Task, send func(masc.Msg)) masc.ComponentOrHTML {
	progress := ""
	percent := 0
	if len(task.Subtasks) > 0 {
		completed := 0
		for _, st := range task.Subtasks {
			if st.Completed {
				completed++
			}
		}
		percent = int(float64(completed) / float64(len(task.Subtasks)) * 100)
		progress = fmt.Sprintf("%d/%d", completed, len(task.Subtasks))
	}

	cardMarkups := []masc.MarkupOrChild{
		masc.Markup(
			masc.Style("cursor", "pointer"),
			event.Click(func(e *masc.Event) {
				send(OpenModal{Mode: ModalDetail, TaskID: task.ID})
			}),
		),
	}
	cardMarkups = append(cardMarkups, p.dragDrop.TaskCardMarkups(task.ID, task.Status, send)...)

	cardChildren := []masc.MarkupOrChild{
		elem.Div(
			masc.Markup(masc.Class("task-header")),
			elem.Div(masc.Markup(masc.Class("task-id")), masc.Text(task.ID)),
			elem.Div(
				masc.Markup(masc.Class("task-actions")),
				elem.Button(
					masc.Markup(
						masc.Class("btn", "btn-ghost"),
						masc.Attribute("title", "Move to top"),
						event.Click(func(e *masc.Event) {
							send(MoveTaskPosition{TaskID: task.ID, Direction: -1})
						}).StopPropagation(),
					),
					masc.Text("⬆️"),
				),
				elem.Button(
					masc.Markup(
						masc.Class("btn", "btn-ghost"),
						masc.Attribute("title", "Move to bottom"),
						event.Click(func(e *masc.Event) {
							send(MoveTaskPosition{TaskID: task.ID, Direction: 1})
						}).StopPropagation(),
					),
					masc.Text("⬇️"),
				),
				elem.Button(
					masc.Markup(
						masc.Class("btn", "btn-ghost"),
						masc.Attribute("title", "Clone task"),
						event.Click(func(e *masc.Event) {
							send(CloneTask{TaskID: task.ID})
						}).StopPropagation(),
					),
					masc.Text("📋"),
				),
				elem.Button(
					masc.Markup(
						masc.Class("btn", "btn-ghost"),
						event.Click(func(e *masc.Event) {
							send(OpenModal{Mode: ModalEdit, TaskID: task.ID})
						}).StopPropagation(),
					),
					masc.Text("✏️"),
				),
			),
		),
		elem.Div(masc.Markup(masc.Class("task-title")), masc.Text(task.Title)),
		masc.If(task.Description != "", elem.Div(
			masc.Markup(
				masc.Class("task-description"),
				masc.UnsafeHTML(markdown.ToHTML(task.Description)),
			),
		)),
		elem.Div(
			append([]masc.MarkupOrChild{masc.Markup(masc.Class("task-meta"))},
				toMarkupChildren(p.taskMetaItems(task))...,
			)...,
		),
		masc.If(task.Modified != "", elem.Div(masc.Markup(masc.Class("task-modified")), masc.Text("Modified: "+task.Modified))),
		masc.If(progress != "", elem.Div(
			masc.Markup(masc.Class("task-subtasks")),
			elem.Div(
				masc.Markup(masc.Class("subtask-progress")),
				elem.Div(
					masc.Markup(masc.Class("progress-bar")),
					elem.Div(masc.Markup(masc.Class("progress-fill"), masc.Style("width", fmt.Sprintf("%d%%", percent))), masc.Text("")),
				),
				elem.Span(masc.Text(progress)),
			),
		)),
	}

	return elem.Div(append(cardMarkups, cardChildren...)...)
}

func (p *Program) taskMetaItems(task Task) []masc.ComponentOrHTML {
	items := make([]masc.ComponentOrHTML, 0, 1+len(task.Assignees)+len(task.Tags))
	if task.Category != "" {
		items = append(items, elem.Span(masc.Markup(masc.Class("badge", "badge-category")), masc.Text(task.Category)))
	}
	items = append(items, p.renderTaskAssignees(task)...)
	items = append(items, p.renderTaskTags(task)...)
	return items
}

func (p *Program) renderTaskAssignees(task Task) []masc.ComponentOrHTML {
	items := make([]masc.ComponentOrHTML, 0, len(task.Assignees))
	for _, assignee := range task.Assignees {
		items = append(items, elem.Span(masc.Markup(masc.Class("badge", "badge-assignee")), masc.Text(assignee)))
	}
	return items
}

func (p *Program) renderTaskTags(task Task) []masc.ComponentOrHTML {
	items := make([]masc.ComponentOrHTML, 0, len(task.Tags))
	for _, tag := range task.Tags {
		items = append(items, elem.Span(masc.Markup(masc.Class("tag")), masc.Text(tag)))
	}
	return items
}

func (p *Program) renderModal(send func(masc.Msg)) masc.ComponentOrHTML {
	if p.modal == ModalNone {
		return nil
	}

	switch p.modal {
	case ModalDetail:
		return p.renderDetailModal(send)
	case ModalEdit:
		return p.renderEditModal(send)
	case ModalArchive:
		return p.renderArchiveModal(send)
	case ModalColumns:
		return p.renderColumnsModal(send)
	case ModalCommit:
		return p.renderCommitModal(send)
	case ModalTodo:
		return p.renderTodoModal(send)
	}
	return nil
}

func (p *Program) renderDetailModal(send func(masc.Msg)) masc.ComponentOrHTML {
	task, ok := p.taskByID(p.detailTaskID)
	if !ok {
		return nil
	}

	statusName := p.statusName(task.Status)
	metaItems := []masc.ComponentOrHTML{
		detailMetaItem("Status", statusName, 1, 1),
	}
	if task.Category != "" {
		metaItems = append(metaItems, detailMetaItem("Category", task.Category, 2, 1))
	}
	if len(task.Assignees) > 0 {
		metaItems = append(metaItems, detailMetaItem("Assigned", strings.Join(task.Assignees, ", "), 3, 1))
	}
	if task.Created != "" {
		metaItems = append(metaItems, detailMetaItem("Creation date", task.Created, 1, 2))
	}
	if task.Modified != "" {
		metaItems = append(metaItems, detailMetaItem("Last modified", task.Modified, 2, 2))
	}
	if task.Completed != "" {
		metaItems = append(metaItems, detailMetaItem("Completed", task.Completed, 3, 2))
	}

	subtaskCompleted := 0
	for _, st := range task.Subtasks {
		if st.Completed {
			subtaskCompleted++
		}
	}

	return elem.Div(
		masc.Markup(
			masc.Class("modal", "active"),
			event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalDetail}) }),
		),
		elem.Div(
			masc.Markup(masc.Class("modal-content"), event.Click(func(e *masc.Event) {}).StopPropagation()),
			elem.Div(
				masc.Markup(masc.Class("modal-header", "task-detail-header")),
				elem.Heading2(masc.Text(task.Title)),
				elem.Div(
					masc.Markup(masc.Class("task-header-right")),
					elem.Span(masc.Markup(masc.Class("task-id-badge")), masc.Text(task.ID)),
					elem.Button(
						masc.Markup(masc.Class("close-btn"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalDetail}) })),
						masc.Text("×"),
					),
				),
			),
			elem.Div(
				masc.Markup(masc.Class("task-detail")),
				elem.Div(
					masc.Markup(masc.Style("padding", "1.5rem")),
					elem.Div(
						masc.Markup(
							masc.Style("margin-bottom", "1.5rem"),
							masc.Style("padding", "1rem"),
							masc.Style("background", "var(--bg)"),
							masc.Style("border-radius", "8px"),
						),
						elem.Div(
							append(
								[]masc.MarkupOrChild{masc.Markup(masc.Class("task-detail-meta"))},
								toMarkupChildren(metaItems)...,
							)...,
						),
					),
					masc.If(len(task.Tags) > 0, elem.Div(
						masc.Markup(masc.Style("margin-bottom", "1.5rem")),
						elem.Div(masc.Markup(
							masc.Style("font-size", "0.85rem"),
							masc.Style("color", "var(--text-secondary)"),
							masc.Style("margin-bottom", "0.5rem"),
						), masc.Text("Tags")),
						elem.Div(
							append(
								[]masc.MarkupOrChild{masc.Markup(
									masc.Style("display", "flex"),
									masc.Style("gap", "0.5rem"),
									masc.Style("flex-wrap", "wrap"),
								)},
								toMarkupChildren(p.renderTaskTags(task))...,
							)...,
						),
					)),
					masc.If(task.Description != "", elem.Div(
						masc.Markup(masc.Style("margin-bottom", "1.5rem")),
						elem.Div(masc.Markup(
							masc.Style("font-size", "0.85rem"),
							masc.Style("color", "var(--text-secondary)"),
							masc.Style("margin-bottom", "0.5rem"),
							masc.Style("font-weight", "600"),
						), masc.Text("Description")),
						elem.Div(masc.Markup(
							masc.Style("line-height", "1.6"),
							masc.Style("color", "var(--text)"),
							masc.UnsafeHTML(markdown.ToHTML(task.Description)),
						)),
					)),
					elem.Div(
						masc.Markup(masc.Style("margin-bottom", "1.5rem")),
						elem.Div(masc.Markup(
							masc.Style("font-size", "0.85rem"),
							masc.Style("color", "var(--text-secondary)"),
							masc.Style("margin-bottom", "0.5rem"),
							masc.Style("font-weight", "600"),
						), masc.Text(fmt.Sprintf("Subtasks (%d/%d)", subtaskCompleted, len(task.Subtasks)))),
						elem.UnorderedList(
							append(
								[]masc.MarkupOrChild{masc.Markup(
									masc.Style("list-style", "none"),
									masc.Style("padding", "0"),
									masc.Style("margin", "0 0 1rem 0"),
								)},
								toMarkupChildren(renderDetailSubtasks(task.ID, task.Subtasks, send))...,
							)...,
						),
						elem.Div(
							masc.Markup(
								masc.Style("display", "flex"),
								masc.Style("gap", "0.5rem"),
								masc.Style("flex-wrap", "wrap"),
							),
							elem.Input(masc.Markup(
								masc.Property("type", "text"),
								masc.Property("placeholder", "New subtask..."),
								masc.Property("value", p.detailSubtaskText),
								masc.Style("flex", "1 1 220px"),
								masc.Style("padding", "0.5rem"),
								masc.Style("border", "2px solid #cbd5e0"),
								masc.Style("border-radius", "4px"),
								masc.Style("font-size", "0.9rem"),
								event.Input(func(e *masc.Event) {
									send(UpdateDetailSubtaskField{Field: "text", Value: e.Target.Get("value").String()})
								}),
								event.KeyDown(func(e *masc.Event) {
									if e.Get("key").String() == "Enter" {
										send(AddTaskSubtask{TaskID: task.ID, Text: p.detailSubtaskText, DueDate: p.detailSubtaskDue})
									}
								}),
							)),
							elem.Input(masc.Markup(
								masc.Property("type", "date"),
								masc.Property("value", p.detailSubtaskDue),
								masc.Style("padding", "0.5rem"),
								masc.Style("border", "2px solid #cbd5e0"),
								masc.Style("border-radius", "4px"),
								masc.Style("font-size", "0.9rem"),
								masc.Style("background", "white"),
								masc.Style("min-width", "160px"),
								event.Input(func(e *masc.Event) {
									send(UpdateDetailSubtaskField{Field: "due", Value: e.Target.Get("value").String()})
								}),
							)),
							elem.Button(
								masc.Markup(masc.Class("btn", "btn-primary"), masc.Style("padding", "0.5rem 1rem"), event.Click(func(e *masc.Event) {
									send(AddTaskSubtask{TaskID: task.ID, Text: p.detailSubtaskText, DueDate: p.detailSubtaskDue})
								})),
								masc.Text("+ Add"),
							),
						),
					),
					masc.If(task.Notes != "", elem.Div(
						masc.Markup(
							masc.Style("margin-top", "1.5rem"),
							masc.Style("padding-top", "1.5rem"),
							masc.Style("border-top", "1px solid #e2e8f0"),
						),
						elem.Div(masc.Markup(
							masc.Style("font-size", "0.85rem"),
							masc.Style("color", "var(--text-secondary)"),
							masc.Style("margin-bottom", "0.75rem"),
							masc.Style("font-weight", "600"),
						), masc.Text("Notes")),
						elem.Div(masc.Markup(
							masc.Style("line-height", "1.7"),
							masc.Style("color", "var(--text)"),
							masc.Style("background", "var(--bg)"),
							masc.Style("padding", "1rem"),
							masc.Style("border-radius", "8px"),
							masc.Style("border-left", "4px solid var(--primary)"),
							masc.UnsafeHTML(markdown.ToHTML(task.Notes)),
						)),
					)),
				),
			),
			elem.Div(
				masc.Markup(masc.Class("actions")),
				elem.Button(
					masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalDetail}) })),
					masc.Text("Close"),
				),
				elem.Button(
					masc.Markup(masc.Class("btn", "btn-danger"), event.Click(func(e *masc.Event) { send(DeleteTask{TaskID: task.ID}) })),
					masc.Text("🗑️ Delete"),
				),
				elem.Button(
					masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(ArchiveTask{TaskID: task.ID}) })),
					masc.Text("📦 Archive"),
				),
				elem.Button(
					masc.Markup(masc.Class("btn", "btn-primary"), event.Click(func(e *masc.Event) { send(OpenModal{Mode: ModalEdit, TaskID: task.ID}) })),
					masc.Text("✏️ Edit"),
				),
			),
		),
	)
}

func (p *Program) renderSubtaskList(subtasks []Subtask) []masc.ComponentOrHTML {
	items := make([]masc.ComponentOrHTML, 0, len(subtasks))
	for _, st := range subtasks {
		suffix := ""
		if st.DueDate != "" {
			suffix = " (due " + st.DueDate + ")"
		}
		label := "[ ] " + st.Text + suffix
		if st.Completed {
			label = "[x] " + st.Text + suffix
		}
		items = append(items, elem.Div(masc.Text(label)))
	}
	return items
}

func detailMetaItem(label, value string, col, row int) masc.ComponentOrHTML {
	return elem.Div(
		masc.Markup(
			masc.Class("task-detail-meta-item"),
			masc.Style("grid-column", fmt.Sprintf("%d", col)),
			masc.Style("grid-row", fmt.Sprintf("%d", row)),
		),
		elem.Div(masc.Markup(
			masc.Style("font-size", "0.85rem"),
			masc.Style("color", "var(--text-secondary)"),
			masc.Style("margin-bottom", "0.25rem"),
		), masc.Text(label)),
		elem.Div(masc.Markup(masc.Style("font-weight", "500")), masc.Text(value)),
	)
}

func renderDetailSubtasks(taskID string, subtasks []Subtask, send func(masc.Msg)) []masc.ComponentOrHTML {
	items := make([]masc.ComponentOrHTML, 0, len(subtasks))
	for idx, st := range subtasks {
		index := idx
		items = append(items, elem.ListItem(
			masc.Markup(
				masc.Style("padding", "0.5rem"),
				masc.Style("margin-bottom", "0.25rem"),
				masc.Style("background", "var(--bg)"),
				masc.Style("border-radius", "4px"),
				masc.Style("display", "flex"),
				masc.Style("align-items", "center"),
				masc.Style("gap", "0.5rem"),
				masc.Style("flex-wrap", "wrap"),
			),
			elem.Input(masc.Markup(
				masc.Property("type", "checkbox"),
				masc.MarkupIf(st.Completed, masc.Property("checked", true)),
				masc.Style("width", "18px"),
				masc.Style("height", "18px"),
				masc.Style("cursor", "pointer"),
				event.Change(func(e *masc.Event) { send(ToggleTaskSubtask{TaskID: taskID, Index: index}) }),
			)),
			elem.Input(masc.Markup(
				masc.Property("type", "text"),
				masc.Property("value", st.Text),
				masc.Style("flex", "1"),
				masc.Style("min-width", "200px"),
				masc.Style("border", "none"),
				masc.Style("background", "transparent"),
				masc.Style("padding", "0"),
				masc.Style("font-size", "0.9rem"),
				masc.Style("outline", "none"),
				masc.MarkupIf(st.Completed,
					masc.Style("text-decoration", "line-through"),
					masc.Style("color", "var(--text-secondary)"),
				),
				event.Input(func(e *masc.Event) {
					send(UpdateTaskSubtaskText{TaskID: taskID, Index: index, Value: e.Target.Get("value").String()})
				}),
			)),
			elem.Input(masc.Markup(
				masc.Property("type", "date"),
				masc.Property("value", st.DueDate),
				masc.Style("padding", "0.35rem"),
				masc.Style("border", "1px solid #cbd5e0"),
				masc.Style("border-radius", "4px"),
				masc.Style("font-size", "0.85rem"),
				masc.Style("background", "white"),
				masc.Style("min-width", "140px"),
				event.Input(func(e *masc.Event) {
					send(UpdateTaskSubtaskDueDate{TaskID: taskID, Index: index, Value: e.Target.Get("value").String()})
				}),
			)),
			elem.Button(
				masc.Markup(masc.Style("background", "none"), masc.Style("border", "none"), masc.Style("cursor", "pointer"), masc.Style("color", "#e53e3e"), masc.Style("font-size", "1.1rem"), masc.Style("padding", "0.25rem"), event.Click(func(e *masc.Event) {
					send(DeleteTaskSubtask{TaskID: taskID, Index: index})
				})),
				masc.Text("🗑️"),
			),
		))
	}
	return items
}

func (p *Program) renderEditModal(send func(masc.Msg)) masc.ComponentOrHTML {
	form := p.form
	isEdit := form.ID != ""

	categories, users, tags := p.uniqueValues()
	sort.Strings(categories)
	sort.Strings(tags)

	userIDs := make([]string, 0, len(users))
	seenUsers := map[string]struct{}{}
	for _, user := range users {
		id := normalizeUserID(user)
		if id == "" {
			continue
		}
		if _, ok := seenUsers[id]; ok {
			continue
		}
		seenUsers[id] = struct{}{}
		userIDs = append(userIDs, id)
	}
	sort.Strings(userIDs)

	tagSuggestions := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		tagSuggestions = append(tagSuggestions, tag)
	}

	categoryPlaceholder := placeholderFromList(categories, 2, ", ")
	assigneePlaceholder := placeholderFromList(userIDs, 2, ", ")
	tagsPlaceholder := placeholderFromList(tagSuggestions, 2, " ")
	tagDropdown := tagSuggestionsForInput(tagSuggestions, form.Tags)
	showTagDropdown := p.tagSuggestionsOpen && len(tagDropdown) > 0
	tagOptionNodes := make([]masc.ComponentOrHTML, 0, len(tagDropdown))
	for _, tag := range tagDropdown {
		tagValue := tag
		tagOptionNodes = append(tagOptionNodes, elem.Div(
			masc.Markup(
				masc.Class("tag-option"),
				event.MouseDown(func(e *masc.Event) {
					send(SelectTagSuggestion{Tag: tagValue})
				}).PreventDefault(),
			),
			elem.Span(
				masc.Markup(masc.Class("tag-pill")),
				masc.Text(tagValue),
			),
		))
	}

	statusOptions := make([]masc.ComponentOrHTML, 0, len(p.config.Columns))
	for _, col := range p.config.Columns {
		statusOptions = append(statusOptions, elem.Option(
			masc.Markup(masc.Property("value", col.ID), masc.MarkupIf(col.ID == form.Status, masc.Property("selected", true))),
			masc.Text(col.Name),
		))
	}

	subtaskItems := make([]masc.ComponentOrHTML, 0, len(form.Subtasks))
	for idx, st := range form.Subtasks {
		index := idx
		subtaskItems = append(subtaskItems, elem.ListItem(
			masc.Markup(
				masc.Style("width", "100%"),
				masc.Style("box-sizing", "border-box"),
				masc.Style("padding", "0.5rem"),
				masc.Style("margin-bottom", "0.25rem"),
				masc.Style("background", "white"),
				masc.Style("border", "1px solid #cbd5e0"),
				masc.Style("border-radius", "4px"),
				masc.Style("display", "flex"),
				masc.Style("align-items", "center"),
				masc.Style("gap", "0.5rem"),
				masc.Style("flex-wrap", "nowrap"),
			),
			elem.Input(masc.Markup(
				masc.Property("type", "checkbox"),
				masc.MarkupIf(st.Completed, masc.Property("checked", true)),
				masc.Style("width", "16px"),
				masc.Style("height", "16px"),
				masc.Style("flex", "0 0 auto"),
				masc.Style("cursor", "pointer"),
				event.Change(func(e *masc.Event) { send(ToggleFormSubtask{Index: index}) }),
			)),
			elem.Input(masc.Markup(
				masc.Property("type", "text"),
				masc.Property("value", st.Text),
				masc.Style("flex", "1 1 0"),
				masc.Style("min-width", "0"),
				masc.Style("padding", "0.35rem"),
				masc.Style("border", "1px solid #cbd5e0"),
				masc.Style("border-radius", "4px"),
				masc.Style("font-size", "0.85rem"),
				masc.Style("background", "white"),
				masc.MarkupIf(st.Completed,
					masc.Style("text-decoration", "line-through"),
					masc.Style("color", "#999"),
				),
				event.Input(func(e *masc.Event) { send(UpdateFormSubtaskText{Index: index, Value: e.Target.Get("value").String()}) }),
			)),
			elem.Input(masc.Markup(
				masc.Property("type", "date"),
				masc.Property("value", st.DueDate),
				masc.Style("flex", "0 0 auto"),
				masc.Style("margin-left", "auto"),
				masc.Style("width", "140px"),
				masc.Style("max-width", "140px"),
				masc.Style("min-width", "140px"),
				masc.Style("box-sizing", "border-box"),
				masc.Style("padding", "0.35rem"),
				masc.Style("border", "1px solid #cbd5e0"),
				masc.Style("border-radius", "4px"),
				masc.Style("font-size", "0.85rem"),
				masc.Style("background", "white"),
				event.Input(func(e *masc.Event) {
					send(UpdateFormSubtaskDueDate{Index: index, Value: e.Target.Get("value").String()})
				}),
			)),
			elem.Button(
				masc.Markup(
					masc.Style("background", "none"),
					masc.Style("border", "none"),
					masc.Style("flex", "0 0 auto"),
					masc.Style("cursor", "pointer"),
					masc.Style("color", "#e53e3e"),
					masc.Style("font-size", "1rem"),
					masc.Style("padding", "0.25rem"),
					event.Click(func(e *masc.Event) { send(DeleteFormSubtask{Index: index}) }),
				),
				masc.Text("🗑️"),
			),
		))
	}

	subtaskChildren := make([]masc.MarkupOrChild, 0, len(subtaskItems)+4)
	subtaskChildren = append(subtaskChildren, masc.Markup(masc.Class("form-group")), elem.Label(masc.Text("Subtasks")))
	subtaskChildren = append(subtaskChildren, elem.UnorderedList(
		append(
			[]masc.MarkupOrChild{masc.Markup(
				masc.Style("list-style", "none"),
				masc.Style("padding", "0"),
				masc.Style("margin", "0 0 0.5rem 0"),
			)},
			toMarkupChildren(subtaskItems)...,
		)...,
	))
	subtaskChildren = append(subtaskChildren,
		elem.Div(
			masc.Markup(
				masc.Style("display", "flex"),
				masc.Style("gap", "0.5rem"),
				masc.Style("flex-wrap", "nowrap"),
				masc.Style("width", "100%"),
				masc.Style("box-sizing", "border-box"),
			),
			elem.Input(masc.Markup(
				masc.Property("type", "text"),
				masc.Property("placeholder", "Add a subtask..."),
				masc.Property("value", p.newSubtaskText),
				masc.Style("flex", "1 1 0"),
				masc.Style("min-width", "0"),
				masc.Style("padding", "0.5rem"),
				masc.Style("border", "2px solid #cbd5e0"),
				masc.Style("border-radius", "4px"),
				masc.Style("font-size", "0.9rem"),
				event.Input(func(e *masc.Event) {
					send(UpdateFormField{Field: "new_subtask_text", Value: e.Target.Get("value").String()})
				}),
			)),
			elem.Input(masc.Markup(
				masc.Property("type", "date"),
				masc.Property("value", p.newSubtaskDue),
				masc.Style("flex", "0 0 auto"),
				masc.Style("margin-left", "auto"),
				masc.Style("width", "140px"),
				masc.Style("max-width", "140px"),
				masc.Style("min-width", "140px"),
				masc.Style("box-sizing", "border-box"),
				masc.Style("padding", "0.5rem"),
				masc.Style("border", "2px solid #cbd5e0"),
				masc.Style("border-radius", "4px"),
				masc.Style("font-size", "0.9rem"),
				masc.Style("background", "white"),
				event.Input(func(e *masc.Event) {
					send(UpdateFormField{Field: "new_subtask_due", Value: e.Target.Get("value").String()})
				}),
			)),
			elem.Button(
				masc.Markup(
					masc.Class("btn", "btn-secondary"),
					masc.Property("type", "button"),
					masc.Style("padding", "0.5rem 1rem"),
					masc.Style("flex", "0 0 auto"),
					event.Click(func(e *masc.Event) {
						if strings.TrimSpace(p.newSubtaskText) == "" {
							return
						}
						send(AddFormSubtask{Text: p.newSubtaskText, DueDate: p.newSubtaskDue})
					}),
				),
				masc.Text("+ Add"),
			),
		),
	)

	return elem.Div(
		masc.Markup(masc.Class("modal", "active")),
		elem.Div(
			masc.Markup(masc.Class("modal-content"), event.Click(func(e *masc.Event) {}).StopPropagation()),
			elem.Div(
				masc.Markup(masc.Class("modal-header")),
				elem.Heading2(masc.Text(func() string {
					if isEdit {
						return "Edit Task"
					}
					return "New Task"
				}())),
				elem.Button(
					masc.Markup(masc.Class("close-btn"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalEdit}) })),
					masc.Text("×"),
				),
			),
			elem.Form(
				masc.Markup(event.Submit(func(e *masc.Event) { send(SaveTask{}) }).PreventDefault()),
				elem.Div(
					masc.Markup(masc.Class("form-group")),
					elem.Label(masc.Text("Title *")),
					elem.Input(masc.Markup(
						masc.Property("type", "text"),
						masc.Property("value", form.Title),
						event.Input(func(e *masc.Event) { send(UpdateFormField{Field: "title", Value: e.Target.Get("value").String()}) }),
					)),
				),
				elem.Div(
					masc.Markup(
						masc.Style("display", "grid"),
						masc.Style("grid-template-columns", "repeat(auto-fit, minmax(180px, 1fr))"),
						masc.Style("gap", "1rem"),
						masc.Style("margin-bottom", "1rem"),
					),
					elem.Div(
						masc.Markup(masc.Class("form-group")),
						elem.Label(masc.Text("Column *")),
						elem.Select(
							append([]masc.MarkupOrChild{
								masc.Markup(event.Change(func(e *masc.Event) { send(UpdateFormField{Field: "status", Value: e.Target.Get("value").String()}) })),
							}, toMarkupChildren(statusOptions)...)...,
						),
					),
					elem.Div(
						masc.Markup(masc.Class("form-group")),
						elem.Label(masc.Text("Category")),
						elem.Input(masc.Markup(
							masc.Property("type", "text"),
							masc.Property("value", form.Category),
							masc.MarkupIf(categoryPlaceholder != "", masc.Property("placeholder", categoryPlaceholder)),
							masc.MarkupIf(len(categories) > 0, masc.Attribute("list", "category-options")),
							event.Input(func(e *masc.Event) { send(UpdateFormField{Field: "category", Value: e.Target.Get("value").String()}) }),
						)),
						masc.If(len(categories) > 0,
							elem.DataList(
								append([]masc.MarkupOrChild{masc.Markup(masc.Attribute("id", "category-options"))}, toMarkupChildren(buildDataListOptions(categories, false))...)...,
							),
						),
					),
					elem.Div(
						masc.Markup(masc.Class("form-group")),
						elem.Label(masc.Text("Assigned to")),
						elem.Input(masc.Markup(
							masc.Property("type", "text"),
							masc.Property("value", form.Assignees),
							masc.MarkupIf(assigneePlaceholder != "", masc.Property("placeholder", assigneePlaceholder)),
							masc.MarkupIf(len(userIDs) > 0, masc.Attribute("list", "assignee-options")),
							event.Input(func(e *masc.Event) { send(UpdateFormField{Field: "assignees", Value: e.Target.Get("value").String()}) }),
						)),
						masc.If(len(userIDs) > 0,
							elem.DataList(
								append([]masc.MarkupOrChild{masc.Markup(masc.Attribute("id", "assignee-options"))}, toMarkupChildren(buildDataListOptions(userIDs, false))...)...,
							),
						),
					),
				),
				elem.Div(
					masc.Markup(
						masc.Style("display", "grid"),
						masc.Style("grid-template-columns", "repeat(auto-fit, minmax(180px, 1fr))"),
						masc.Style("gap", "1rem"),
						masc.Style("margin-bottom", "1rem"),
					),
					elem.Div(
						masc.Markup(masc.Class("form-group")),
						elem.Label(masc.Text("Created")),
						elem.Input(masc.Markup(
							masc.Property("type", "date"),
							masc.Property("value", form.Created),
							event.Input(func(e *masc.Event) { send(UpdateFormField{Field: "created", Value: e.Target.Get("value").String()}) }),
						)),
					),
					elem.Div(
						masc.Markup(masc.Class("form-group")),
						elem.Label(masc.Text("Completed")),
						elem.Input(masc.Markup(
							masc.Property("type", "date"),
							masc.Property("value", form.Completed),
							event.Input(func(e *masc.Event) { send(UpdateFormField{Field: "completed", Value: e.Target.Get("value").String()}) }),
						)),
					),
				),
				elem.Div(
					masc.Markup(masc.Class("form-group")),
					elem.Label(masc.Text("Tags")),
					elem.Input(masc.Markup(
						masc.Property("type", "text"),
						masc.Property("value", form.Tags),
						masc.MarkupIf(tagsPlaceholder != "", masc.Property("placeholder", tagsPlaceholder)),
						event.Input(func(e *masc.Event) { send(UpdateFormField{Field: "tags", Value: e.Target.Get("value").String()}) }),
						event.Focus(func(e *masc.Event) { send(SetTagSuggestionsOpen{Open: true}) }),
						event.FocusOut(func(e *masc.Event) { send(SetTagSuggestionsOpen{Open: false}) }),
					)),
					masc.If(showTagDropdown,
						elem.Div(
							append([]masc.MarkupOrChild{masc.Markup(masc.Class("tags-autocomplete"))}, toMarkupChildren(tagOptionNodes)...)...,
						),
					),
				),
				elem.Div(
					masc.Markup(masc.Class("form-group")),
					elem.Label(masc.Text("Description")),
					elem.TextArea(masc.Markup(
						masc.Property("value", form.Description),
						event.Input(func(e *masc.Event) {
							send(UpdateFormField{Field: "description", Value: e.Target.Get("value").String()})
						}),
					)),
				),
				elem.Div(
					masc.Markup(masc.Class("form-group")),
					elem.Label(masc.Text("Notes")),
					elem.TextArea(masc.Markup(
						masc.Property("value", form.Notes),
						event.Input(func(e *masc.Event) { send(UpdateFormField{Field: "notes", Value: e.Target.Get("value").String()}) }),
					)),
				),
				elem.Div(subtaskChildren...),
				elem.Div(
					masc.Markup(masc.Class("actions")),
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-secondary"), masc.Property("type", "button"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalEdit}) })),
						masc.Text("Cancel"),
					),
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-primary"), masc.Property("type", "submit")),
						masc.Text(func() string {
							if isEdit {
								return "Save"
							}
							return "Create"
						}()),
					),
				),
			),
		),
	)
}

func (p *Program) renderArchiveModal(send func(masc.Msg)) masc.ComponentOrHTML {
	search := strings.TrimSpace(p.archiveSearch)
	filtered := make([]Task, 0, len(p.archived))
	for _, task := range p.archived {
		if search == "" || strings.Contains(strings.ToLower(task.Title), strings.ToLower(search)) || strings.Contains(strings.ToLower(task.Description), strings.ToLower(search)) || strings.Contains(strings.ToLower(task.Category), strings.ToLower(search)) || strings.Contains(strings.ToLower(strings.Join(task.Tags, " ")), strings.ToLower(search)) {
			filtered = append(filtered, task)
		}
	}

	items := make([]masc.ComponentOrHTML, 0, len(filtered))
	for _, task := range filtered {
		pills := make([]masc.ComponentOrHTML, 0, 1+len(task.Tags))
		if task.Category != "" {
			pills = append(pills, elem.Span(masc.Markup(masc.Class("archive-pill", "archive-pill-category")), masc.Text(task.Category)))
		}
		for _, tag := range task.Tags {
			pills = append(pills, elem.Span(masc.Markup(masc.Class("archive-pill", "archive-pill-tag")), masc.Text(tag)))
		}

		items = append(items, elem.Div(
			masc.Markup(masc.Class("archive-item")),
			elem.Div(
				masc.Markup(masc.Class("archive-item-header")),
				elem.Div(
					masc.Markup(masc.Class("archive-title")),
					elem.Span(masc.Markup(masc.Class("archive-id")), masc.Text(task.ID)),
					elem.Strong(masc.Text(task.Title)),
				),
				elem.Div(
					masc.Markup(masc.Class("archive-actions")),
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-secondary", "archive-delete"), event.Click(func(e *masc.Event) { send(DeleteTask{TaskID: task.ID}) })),
						masc.Text("🗑️"),
					),
					elem.Button(
						masc.Markup(masc.Class("btn", "btn-primary", "archive-restore"), event.Click(func(e *masc.Event) { send(RestoreTask{TaskID: task.ID}) })),
						masc.Text("↩️ Restore"),
					),
				),
			),
			masc.If(task.Description != "", elem.Div(
				masc.Markup(masc.Class("archive-description"), masc.UnsafeHTML(markdown.ToHTML(task.Description))),
			)),
			masc.If(len(pills) > 0, elem.Div(
				append([]masc.MarkupOrChild{masc.Markup(masc.Class("archive-tags"))}, toMarkupChildren(pills)...)...,
			)),
		))
	}

	if len(items) == 0 {
		items = append(items, elem.Div(masc.Markup(masc.Class("archive-empty")), masc.Text("No archived tasks.")))
	}

	archiveContent := make([]masc.MarkupOrChild, 0, len(items)+3)
	archiveContent = append(archiveContent,
		masc.Markup(masc.Class("modal-content", "archive-modal"), event.Click(func(e *masc.Event) {}).StopPropagation()),
		elem.Div(
			masc.Markup(masc.Class("modal-header")),
			elem.Heading2(masc.Text("📦 Archives")),
			elem.Button(
				masc.Markup(masc.Class("close-btn"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalArchive}) })),
				masc.Text("×"),
			),
		),
		elem.Div(
			masc.Markup(masc.Class("archive-body")),
			elem.Div(
				masc.Markup(masc.Class("archive-search")),
				elem.Input(masc.Markup(
					masc.Class("archive-search-input"),
					masc.Property("type", "text"),
					masc.Property("placeholder", "Search in archives..."),
					masc.Property("value", p.archiveSearch),
					event.Input(func(e *masc.Event) { send(UpdateArchiveSearch{Value: e.Target.Get("value").String()}) }),
				)),
			),
			elem.Div(
				append([]masc.MarkupOrChild{masc.Markup(masc.Class("archive-list"))}, toMarkupChildren(items)...)...,
			),
		),
	)

	return elem.Div(
		masc.Markup(
			masc.Class("modal", "active"),
			event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalArchive}) }),
		),
		elem.Div(archiveContent...),
	)
}

func (p *Program) renderColumnsModal(send func(masc.Msg)) masc.ComponentOrHTML {
	items := make([]masc.ComponentOrHTML, 0, len(p.config.Columns))
	for idx, col := range p.config.Columns {
		index := idx
		items = append(items, elem.Div(
			masc.Markup(masc.Class("summary-card")),
			elem.Div(
				masc.Markup(masc.Style("display", "grid"), masc.Style("gap", "0.5rem"), masc.Style("grid-template-columns", "auto 1fr 1fr auto"), masc.Style("align-items", "center")),
				elem.Div(
					elem.Button(masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(MoveColumn{Index: index, Direction: -1}) })), masc.Text("↑")),
					elem.Button(masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(MoveColumn{Index: index, Direction: 1}) })), masc.Text("↓")),
				),
				elem.Input(masc.Markup(
					masc.Property("type", "text"),
					masc.Property("value", col.Name),
					event.Input(func(e *masc.Event) {
						send(UpdateColumn{Index: index, Field: "name", Value: e.Target.Get("value").String()})
					}),
				)),
				elem.Input(masc.Markup(
					masc.Property("type", "text"),
					masc.Property("value", col.ID),
					event.Input(func(e *masc.Event) {
						send(UpdateColumn{Index: index, Field: "id", Value: e.Target.Get("value").String()})
					}),
				)),
				elem.Button(
					masc.Markup(
						masc.Class("btn", "btn-ghost"),
						masc.Style("color", "#e53e3e"),
						event.Click(func(e *masc.Event) { send(DeleteColumn{Index: index}) }),
					),
					masc.Text("🗑️"),
				),
			),
		))
	}

	columnsContent := make([]masc.MarkupOrChild, 0, len(items)+3)
	columnsContent = append(columnsContent,
		masc.Markup(masc.Class("modal-content"), event.Click(func(e *masc.Event) {}).StopPropagation()),
		elem.Div(
			masc.Markup(masc.Class("modal-header")),
			elem.Heading2(masc.Text("Manage Columns")),
			elem.Button(masc.Markup(masc.Class("close-btn"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalColumns}) })), masc.Text("×")),
		),
		elem.Div(
			masc.Markup(masc.Class("actions")),
			elem.Button(masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(AddColumn{}) })), masc.Text("+ Add Column")),
		),
	)
	columnsContent = append(columnsContent, toMarkupChildren(items)...)

	return elem.Div(
		masc.Markup(
			masc.Class("modal", "active"),
			event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalColumns}) }),
		),
		elem.Div(columnsContent...),
	)
}

func (p *Program) renderCommitModal(send func(masc.Msg)) masc.ComponentOrHTML {
	diffText := p.commitDiff
	if strings.TrimSpace(diffText) == "" {
		diffText = "No changes to commit."
	}
	hasChanges := p.hasPendingCommitChanges()
	missingBlankLine := commitMessageMissingBlankLine(p.commitMessageDraft)

	return elem.Div(
		masc.Markup(
			masc.Class("modal", "active"),
			event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalCommit}) }),
		),
		elem.Div(
			masc.Markup(masc.Class("modal-content"), event.Click(func(e *masc.Event) {}).StopPropagation()),
			elem.Div(
				masc.Markup(masc.Class("modal-header")),
				elem.Heading2(masc.Text("Commit Changes")),
				elem.Button(
					masc.Markup(masc.Class("close-btn"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalCommit}) })),
					masc.Text("×"),
				),
			),
			elem.Div(
				masc.Markup(masc.Class("form-group")),
				elem.Label(masc.Text("Commit message")),
				elem.TextArea(masc.Markup(
					masc.Property("rows", "3"),
					masc.Property("value", p.commitMessageDraft),
					event.Input(func(e *masc.Event) { send(UpdateCommitMessage{Value: e.Target.Get("value").String()}) }),
				)),
				masc.If(missingBlankLine,
					elem.Div(
						masc.Markup(masc.Class("form-warning")),
						masc.Text("Add a blank line between the subject and body."),
					),
				),
			),
			elem.Div(
				masc.Markup(masc.Class("form-group")),
				elem.Label(masc.Text("Diff")),
				elem.Preformatted(append(
					[]masc.MarkupOrChild{masc.Markup(masc.Class("diff-view"))},
					renderDiffLines(diffText)...,
				)...),
			),
			elem.Div(
				masc.Markup(masc.Class("actions")),
				elem.Button(
					masc.Markup(masc.Class("btn", "btn-secondary"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalCommit}) })),
					masc.Text("Cancel"),
				),
				elem.Button(
					masc.Markup(
						masc.Class("btn", "btn-primary"),
						masc.Property("disabled", !hasChanges || p.commitInProgress || strings.TrimSpace(p.commitMessageDraft) == "" || missingBlankLine),
						event.Click(func(e *masc.Event) { send(ConfirmCommit{Message: p.commitMessageDraft}) }),
					),
					masc.Text("Commit"),
				),
			),
		),
	)
}

func renderDiffLines(diffText string) []masc.MarkupOrChild {
	diffText = strings.ReplaceAll(diffText, "\r\n", "\n")
	lines := strings.Split(diffText, "\n")
	out := make([]masc.MarkupOrChild, 0, len(lines))
	for _, line := range lines {
		class := diffLineClass(line)
		display := line
		if display == "" {
			display = " "
		}
		out = append(out, elem.Span(
			masc.Markup(masc.Class("diff-line", class)),
			masc.Text(display),
		))
	}
	return out
}

func diffLineClass(line string) string {
	switch {
	case strings.HasPrefix(line, "@@"):
		return "diff-hunk"
	case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
		return "diff-file"
	case strings.HasPrefix(line, "+"):
		return "diff-add"
	case strings.HasPrefix(line, "-"):
		return "diff-del"
	default:
		return "diff-context"
	}
}

type todoItem struct {
	TaskID          string
	TaskTitle       string
	TaskAssignees   []string
	SubtaskIndex    int
	SubtaskText     string
	SubtaskDueDate  string
	SubtaskSortDate string
}

func (p *Program) renderTodoModal(send func(masc.Msg)) masc.ComponentOrHTML {
	items := p.buildTodoItems()
	listChildren := make([]masc.ComponentOrHTML, 0, len(items)*2)

	if len(items) == 0 {
		listChildren = append(listChildren, elem.Paragraph(
			masc.Markup(masc.Class("todo-empty")),
			masc.Text("No incomplete subtasks"),
		))
	} else {
		currentTaskID := ""
		var currentGroup []masc.ComponentOrHTML
		for _, item := range items {
			itemValue := item
			if itemValue.TaskID != currentTaskID {
				if len(currentGroup) > 0 {
					listChildren = append(listChildren, elem.Div(
						append([]masc.MarkupOrChild{masc.Markup(masc.Class("todo-task-group"))}, toMarkupChildren(currentGroup)...)...,
					))
					currentGroup = nil
				}
				currentTaskID = itemValue.TaskID
				assigneeText := strings.Join(itemValue.TaskAssignees, ", ")
				taskHeaderChildren := []masc.ComponentOrHTML{
					elem.Span(masc.Text(itemValue.TaskTitle)),
					elem.Span(masc.Markup(masc.Class("todo-task-id")), masc.Text("("+itemValue.TaskID+")")),
				}
				if assigneeText != "" {
					taskHeaderChildren = append(taskHeaderChildren, elem.Span(masc.Markup(masc.Class("todo-task-assignees")), masc.Text(assigneeText)))
				}
				currentGroup = append(currentGroup, elem.Button(
					append([]masc.MarkupOrChild{
						masc.Markup(
							masc.Class("todo-task-header"),
							masc.Property("type", "button"),
							event.MouseDown(func(e *masc.Event) {
								send(OpenDetailFromTodo{TaskID: itemValue.TaskID})
							}).PreventDefault(),
						),
					}, toMarkupChildren(taskHeaderChildren)...)...,
				))
			}

			subtaskChildren := []masc.ComponentOrHTML{
				elem.Input(masc.Markup(
					masc.Property("type", "checkbox"),
					masc.Style("cursor", "pointer"),
					event.Change(func(e *masc.Event) { send(ToggleTaskSubtask{TaskID: itemValue.TaskID, Index: itemValue.SubtaskIndex}) }),
				)),
				elem.Span(masc.Markup(masc.Class("todo-subtask-text")), masc.Text(itemValue.SubtaskText)),
			}
			if itemValue.SubtaskDueDate != "" {
				subtaskChildren = append(subtaskChildren, elem.Span(
					masc.Markup(masc.Class("todo-due")),
					masc.Text("Due: "+itemValue.SubtaskDueDate),
				))
			}
			currentGroup = append(currentGroup, elem.Div(
				append([]masc.MarkupOrChild{masc.Markup(masc.Class("todo-subtask"))}, toMarkupChildren(subtaskChildren)...)...,
			))
		}
		if len(currentGroup) > 0 {
			listChildren = append(listChildren, elem.Div(
				append([]masc.MarkupOrChild{masc.Markup(masc.Class("todo-task-group"))}, toMarkupChildren(currentGroup)...)...,
			))
		}
	}

	return elem.Div(
		masc.Markup(
			masc.Class("modal", "active"),
			event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalTodo}) }),
		),
		elem.Div(
			masc.Markup(masc.Class("modal-content", "todo-modal"), masc.Style("max-width", "700px"), event.Click(func(e *masc.Event) {}).StopPropagation()),
			elem.Div(
				masc.Markup(masc.Class("modal-header")),
				elem.Heading2(masc.Text("📋 TODO")),
				elem.Button(
					masc.Markup(masc.Class("close-btn"), event.Click(func(e *masc.Event) { send(CloseModal{Mode: ModalTodo}) })),
					masc.Text("×"),
				),
			),
			elem.Div(
				masc.Markup(masc.Class("todo-body")),
				elem.Div(
					append([]masc.MarkupOrChild{masc.Markup(masc.Class("todo-list"))}, toMarkupChildren(listChildren)...)...,
				),
			),
		),
	)
}

func commitMessageMissingBlankLine(message string) bool {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	if len(lines) <= 1 {
		return false
	}
	if strings.TrimSpace(lines[0]) == "" {
		return false
	}
	return strings.TrimSpace(lines[1]) != ""
}

func (p *Program) hasPendingCommitChanges() bool {
	return p.lastSavedKanban != p.generateKanbanMarkdown() || p.lastSavedArchive != p.generateArchiveMarkdown()
}

func (p *Program) buildTodoItems() []todoItem {
	items := make([]todoItem, 0)
	for _, task := range p.tasks {
		if len(task.Subtasks) == 0 {
			continue
		}
		sortDate := task.Modified
		if sortDate == "" {
			sortDate = task.Created
		}
		if sortDate == "" {
			sortDate = "9999-99-99"
		}
		for idx, st := range task.Subtasks {
			if st.Completed {
				continue
			}
			items = append(items, todoItem{
				TaskID:          task.ID,
				TaskTitle:       task.Title,
				TaskAssignees:   task.Assignees,
				SubtaskIndex:    idx,
				SubtaskText:     st.Text,
				SubtaskDueDate:  normalizeDueDate(st.DueDate),
				SubtaskSortDate: sortDate,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		a := items[i]
		b := items[j]
		aHasDue := a.SubtaskDueDate != ""
		bHasDue := b.SubtaskDueDate != ""
		if aHasDue != bHasDue {
			return aHasDue
		}
		if aHasDue && bHasDue && a.SubtaskDueDate != b.SubtaskDueDate {
			return a.SubtaskDueDate < b.SubtaskDueDate
		}
		if a.SubtaskSortDate != b.SubtaskSortDate {
			return a.SubtaskSortDate < b.SubtaskSortDate
		}
		if a.TaskTitle != b.TaskTitle {
			return a.TaskTitle < b.TaskTitle
		}
		if a.TaskID != b.TaskID {
			return a.TaskID < b.TaskID
		}
		return a.SubtaskIndex < b.SubtaskIndex
	})

	return items
}

func placeholderFromList(values []string, limit int, sep string) string {
	if len(values) == 0 || limit <= 0 {
		return ""
	}
	if len(values) < limit {
		limit = len(values)
	}
	text := strings.Join(values[:limit], sep)
	if len(values) > limit {
		text += "..."
	}
	return text
}

func buildDataListOptions(values []string, ensureHashtag bool) []masc.ComponentOrHTML {
	options := make([]masc.ComponentOrHTML, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if ensureHashtag && !strings.HasPrefix(value, "#") {
			value = "#" + value
		}
		options = append(options, elem.Option(
			masc.Markup(masc.Property("value", value)),
			masc.Text(value),
		))
	}
	return options
}

func tagSuggestionsForInput(allTags []string, input string) []string {
	lastWord := lastTagWord(input)
	currentTags := strings.Fields(input)
	currentLookup := map[string]struct{}{}
	for _, tag := range currentTags {
		currentLookup[strings.ToLower(tag)] = struct{}{}
	}

	out := make([]string, 0, len(allTags))
	lastLower := strings.ToLower(lastWord)
	for _, tag := range allTags {
		clean := strings.TrimSpace(tag)
		if clean == "" {
			continue
		}
		tagLower := strings.ToLower(clean)
		matchesPrefix := lastLower == "" || strings.HasPrefix(tagLower, lastLower)
		if !matchesPrefix {
			continue
		}
		_, already := currentLookup[tagLower]
		if already && !(lastLower != "" && strings.HasPrefix(tagLower, lastLower)) {
			continue
		}
		out = append(out, clean)
	}
	return out
}

func lastTagWord(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	if strings.HasSuffix(input, " ") || strings.HasSuffix(input, "\t") || strings.HasSuffix(input, "\n") {
		return ""
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func applyTagSuggestion(input string, tag string) string {
	if strings.TrimSpace(tag) == "" {
		return input
	}
	if strings.TrimSpace(input) == "" {
		return tag + " "
	}
	if strings.HasSuffix(input, " ") || strings.HasSuffix(input, "\t") || strings.HasSuffix(input, "\n") {
		return strings.TrimRight(input, " \t\r\n") + " " + tag + " "
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return tag + " "
	}
	parts[len(parts)-1] = tag
	return strings.Join(parts, " ") + " "
}

func (p *Program) renderError() masc.ComponentOrHTML {
	return elem.Div(
		masc.Markup(masc.Class("summary-card"), masc.Style("border-left", "4px solid var(--danger)")),
		masc.Text(p.error),
	)
}

func (p *Program) renderStatus() masc.ComponentOrHTML {
	return elem.Div(
		masc.Markup(masc.Class("summary-card")),
		masc.Text(p.status),
	)
}

func (p *Program) setStatus(message string, autoClear bool) masc.Cmd {
	p.statusSeq++
	p.status = message
	if message == "" || !autoClear {
		return nil
	}
	seq := p.statusSeq
	return func() masc.Msg {
		time.Sleep(5 * time.Second)
		return ClearStatus{Seq: seq}
	}
}

func (p *Program) renderLoadingOverlay() masc.ComponentOrHTML {
	if !p.loading {
		return nil
	}
	return elem.Div(
		masc.Markup(masc.Class("loading-overlay")),
		elem.Div(
			masc.Markup(masc.Class("loading")),
			masc.Text("Loading…"),
		),
	)
}

func (p *Program) renderNotification() masc.ComponentOrHTML {
	if p.error != "" {
		return elem.Div(masc.Markup(masc.Class("notification", "error", "show")), masc.Text(p.error))
	}
	if p.status != "" && !p.loading {
		const commitPrefix = "Committed: "
		if strings.HasPrefix(p.status, commitPrefix) {
			url := strings.TrimSpace(strings.TrimPrefix(p.status, commitPrefix))
			if url != "" {
				return elem.Div(
					masc.Markup(masc.Class("notification", "success", "show")),
					elem.Span(masc.Text("Committed: ")),
					elem.Anchor(
						masc.Markup(
							masc.Attribute("href", url),
							masc.Attribute("target", "_blank"),
							masc.Attribute("rel", "noreferrer"),
						),
						masc.Text(url),
					),
				)
			}
		}
		return elem.Div(masc.Markup(masc.Class("notification", "success", "show")), masc.Text(p.status))
	}
	return nil
}

func (p *Program) handleSaveTask() (masc.Model, masc.Cmd) {
	form := p.form
	title := strings.TrimSpace(form.Title)
	status := strings.TrimSpace(form.Status)
	if title == "" {
		p.error = "Title is required."
		return p, nil
	}
	if status == "" {
		p.error = "Column is required."
		return p, nil
	}
	assignees := parseAssignees(form.Assignees)
	tags := parseTags(form.Tags)
	today := time.Now().Format("2006-01-02")

	if form.ID != "" {
		idx := p.taskIndexByID(form.ID)
		if idx < 0 {
			p.error = "Task not found."
			return p, nil
		}
		task := p.tasks[idx]
		task.Title = title
		task.Status = status
		task.Category = strings.TrimSpace(form.Category)
		task.Assignees = assignees
		task.Tags = tags
		task.Created = strings.TrimSpace(form.Created)
		task.Modified = today
		task.Completed = strings.TrimSpace(form.Completed)
		task.Description = strings.TrimSpace(form.Description)
		task.Subtasks = form.Subtasks
		task.Notes = strings.TrimSpace(form.Notes)
		p.tasks[idx] = task
		statusCmd := p.setStatus("Task updated.", true)
		p.dirty = true
		p.modal = ModalNone
		p.form = TaskForm{}
		return p, statusCmd
	} else {
		newTask := Task{
			ID:          generateTaskID(),
			Title:       title,
			Status:      status,
			Category:    strings.TrimSpace(form.Category),
			Assignees:   assignees,
			Tags:        tags,
			Created:     firstNonEmpty(strings.TrimSpace(form.Created), today),
			Modified:    today,
			Completed:   strings.TrimSpace(form.Completed),
			Description: strings.TrimSpace(form.Description),
			Subtasks:    form.Subtasks,
			Notes:       strings.TrimSpace(form.Notes),
		}
		p.tasks = append(p.tasks, newTask)
		statusCmd := p.setStatus("Task created.", true)
		p.dirty = true
		p.modal = ModalNone
		p.form = TaskForm{}
		return p, statusCmd
	}
}

func (p *Program) deleteTaskByID(taskID string) {
	idx := p.taskIndexByID(taskID)
	if idx < 0 {
		return
	}
	p.tasks = append(p.tasks[:idx], p.tasks[idx+1:]...)
	p.dirty = true
	p.modal = ModalNone
}

func (p *Program) archiveTask(taskID string) {
	idx := p.taskIndexByID(taskID)
	if idx < 0 {
		return
	}
	task := p.tasks[idx]
	p.tasks = append(p.tasks[:idx], p.tasks[idx+1:]...)
	p.archived = append(p.archived, task)
	p.dirty = true
	p.modal = ModalNone
}

func (p *Program) restoreTask(taskID string) {
	idx := p.archivedIndexByID(taskID)
	if idx < 0 {
		return
	}
	task := p.archived[idx]
	p.archived = append(p.archived[:idx], p.archived[idx+1:]...)
	if !p.columnExists(task.Status) {
		task.Status = p.firstColumnID()
	}
	p.tasks = append(p.tasks, task)
	p.dirty = true
}

func (p *Program) cloneTask(taskID string) (*Program, masc.Cmd) {
	idx := p.taskIndexByID(taskID)
	if idx < 0 {
		return p, nil
	}
	original := p.tasks[idx]
	today := time.Now().Format("2006-01-02")
	cloned := Task{
		ID:          generateTaskID(),
		Title:       original.Title,
		Status:      original.Status,
		Category:    original.Category,
		Assignees:   append([]string{}, original.Assignees...),
		Tags:        append([]string{}, original.Tags...),
		Created:     today,
		Modified:    today,
		Completed:   "",
		Description: original.Description,
		Subtasks:    cloneSubtasks(original.Subtasks),
		Notes:       original.Notes,
	}
	newTasks := make([]Task, 0, len(p.tasks)+1)
	newTasks = append(newTasks, p.tasks[:idx+1]...)
	newTasks = append(newTasks, cloned)
	newTasks = append(newTasks, p.tasks[idx+1:]...)
	p.tasks = newTasks
	p.dirty = true
	p.modal = ModalEdit
	p.form = p.formForTask(cloned.ID)
	p.newSubtaskText = ""
	p.newSubtaskDue = ""
	p.tagSuggestionsOpen = false
	return p, p.setStatus("Task cloned.", true)
}

func cloneSubtasks(subtasks []Subtask) []Subtask {
	cloned := make([]Subtask, len(subtasks))
	for i, s := range subtasks {
		cloned[i] = Subtask{
			Completed: false,
			Text:      s.Text,
			DueDate:   s.DueDate,
		}
	}
	return cloned
}

func (p *Program) moveTaskWithinColumn(taskID string, direction int) {
	idx := p.taskIndexByID(taskID)
	if idx < 0 {
		return
	}
	task := p.tasks[idx]
	columnIndices := make([]int, 0)
	for i, t := range p.tasks {
		if t.Status == task.Status {
			columnIndices = append(columnIndices, i)
		}
	}
	if len(columnIndices) < 2 {
		return
	}
	lastInColumn := columnIndices[len(columnIndices)-1]
	targetIdx := lastInColumn
	if direction < 0 {
		if idx == columnIndices[0] {
			return
		}
		targetIdx = columnIndices[0]
	} else {
		if idx == lastInColumn {
			return
		}
		targetIdx = lastInColumn + 1
	}
	if targetIdx == idx {
		return
	}

	task = p.tasks[idx]
	p.tasks = append(p.tasks[:idx], p.tasks[idx+1:]...)
	if idx < targetIdx {
		targetIdx--
	}
	if targetIdx < 0 {
		targetIdx = 0
	}
	if targetIdx > len(p.tasks) {
		targetIdx = len(p.tasks)
	}
	p.tasks = append(p.tasks, Task{})
	copy(p.tasks[targetIdx+1:], p.tasks[targetIdx:])
	p.tasks[targetIdx] = task
	p.dirty = true
}

func (p *Program) handleDropOnTask(targetTaskID, columnID string) {
	dragged := strings.TrimSpace(p.dragDrop.DraggingTaskID)
	if dragged == "" || dragged == targetTaskID {
		return
	}
	p.moveTaskByDrop(dragged, targetTaskID, columnID)
}

func (p *Program) handleDropOnColumn(columnID string) {
	dragged := strings.TrimSpace(p.dragDrop.DraggingTaskID)
	if dragged == "" {
		return
	}
	p.moveTaskByDrop(dragged, "", columnID)
}

func (p *Program) moveTaskByDrop(taskID, targetTaskID, targetColumnID string) {
	idx := p.taskIndexByID(taskID)
	if idx < 0 {
		return
	}
	if targetColumnID == "" {
		return
	}
	if !p.columnExists(targetColumnID) {
		return
	}

	task := p.tasks[idx]
	task.Status = targetColumnID
	task.Modified = time.Now().Format("2006-01-02")
	p.tasks = append(p.tasks[:idx], p.tasks[idx+1:]...)

	insertIdx := p.dropInsertIndex(targetTaskID, targetColumnID)
	if insertIdx < 0 {
		insertIdx = 0
	}
	if insertIdx > len(p.tasks) {
		insertIdx = len(p.tasks)
	}
	p.tasks = append(p.tasks, Task{})
	copy(p.tasks[insertIdx+1:], p.tasks[insertIdx:])
	p.tasks[insertIdx] = task
	p.dirty = true
}

func (p *Program) dropInsertIndex(targetTaskID, targetColumnID string) int {
	if targetTaskID != "" {
		if idx := p.taskIndexByID(targetTaskID); idx >= 0 {
			return idx
		}
	}
	lastIndex := -1
	for i := len(p.tasks) - 1; i >= 0; i-- {
		if p.tasks[i].Status == targetColumnID {
			lastIndex = i
			break
		}
	}
	if lastIndex >= 0 {
		return lastIndex + 1
	}

	columnIndex := p.columnOrderIndex(targetColumnID)
	for i, task := range p.tasks {
		if p.columnOrderIndex(task.Status) > columnIndex {
			return i
		}
	}
	return len(p.tasks)
}

func (p *Program) columnOrderIndex(columnID string) int {
	for i, col := range p.config.Columns {
		if col.ID == columnID {
			return i
		}
	}
	return len(p.config.Columns)
}

func (p *Program) addColumn() {
	p.config.Columns = append(p.config.Columns, Column{Name: "New Column", ID: fmt.Sprintf("column-%d", len(p.config.Columns)+1)})
	p.dirty = true
}

func (p *Program) updateColumn(index int, field, value string) {
	if index < 0 || index >= len(p.config.Columns) {
		return
	}
	switch field {
	case "name":
		p.config.Columns[index].Name = value
	case "id":
		p.config.Columns[index].ID = value
	}
	p.dirty = true
}

func (p *Program) deleteColumn(index int) {
	if index < 0 || index >= len(p.config.Columns) {
		return
	}
	p.config.Columns = append(p.config.Columns[:index], p.config.Columns[index+1:]...)
	p.dirty = true
}

func (p *Program) moveColumn(index int, direction int) {
	newIdx := index + direction
	if newIdx < 0 || newIdx >= len(p.config.Columns) {
		return
	}
	p.config.Columns[index], p.config.Columns[newIdx] = p.config.Columns[newIdx], p.config.Columns[index]
	p.dirty = true
}

func (p *Program) formForTask(taskID string) TaskForm {
	if taskID == "" {
		form := TaskForm{}
		if len(p.config.Columns) > 0 {
			form.Status = p.config.Columns[0].ID
		}
		return form
	}
	task, ok := p.taskByID(taskID)
	if !ok {
		return TaskForm{}
	}
	return TaskForm{
		ID:          task.ID,
		Title:       task.Title,
		Status:      task.Status,
		Category:    task.Category,
		Assignees:   strings.Join(task.Assignees, ", "),
		Tags:        strings.Join(task.Tags, " "),
		Created:     task.Created,
		Completed:   task.Completed,
		Description: task.Description,
		Notes:       task.Notes,
		Subtasks:    append([]Subtask{}, task.Subtasks...),
	}
}

func (p *Program) taskByID(taskID string) (Task, bool) {
	for _, task := range p.tasks {
		if task.ID == taskID {
			return task, true
		}
	}
	return Task{}, false
}

func (p *Program) taskIndexByID(taskID string) int {
	for i, task := range p.tasks {
		if task.ID == taskID {
			return i
		}
	}
	return -1
}

func (p *Program) updateTaskSubtasks(taskID string, update func(*Task)) {
	idx := p.taskIndexByID(taskID)
	if idx < 0 {
		return
	}
	task := p.tasks[idx]
	update(&task)
	task.Modified = time.Now().Format("2006-01-02")
	p.tasks[idx] = task
	p.dirty = true
}

func (p *Program) archivedIndexByID(taskID string) int {
	for i, task := range p.archived {
		if task.ID == taskID {
			return i
		}
	}
	return -1
}

func (p *Program) columnExists(id string) bool {
	for _, col := range p.config.Columns {
		if col.ID == id {
			return true
		}
	}
	return false
}

func (p *Program) firstColumnID() string {
	if len(p.config.Columns) == 0 {
		return "todo"
	}
	return p.config.Columns[0].ID
}

func (p *Program) statusName(id string) string {
	for _, col := range p.config.Columns {
		if col.ID == id {
			return col.Name
		}
	}
	return id
}

func (p *Program) filteredTasks(columnID string) []Task {
	filtered := make([]Task, 0)
	for _, task := range p.tasks {
		if task.Status != columnID {
			continue
		}
		if !p.matchesFilters(task) {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered
}

func (p *Program) matchesFilters(task Task) bool {
	if len(p.filters) > 0 {
		byType := map[string][]string{}
		for _, f := range p.filters {
			byType[f.Type] = append(byType[f.Type], f.Value)
		}
		for ftype, values := range byType {
			matchesType := false
			switch ftype {
			case "tag":
				for _, value := range values {
					if hasTag(task.Tags, value) {
						matchesType = true
						break
					}
				}
			case "category":
				for _, value := range values {
					if task.Category == value {
						matchesType = true
						break
					}
				}
			case "user":
				for _, value := range values {
					if value == "<unassigned>" {
						if len(task.Assignees) == 0 {
							matchesType = true
							break
						}
					} else if hasAssignee(task.Assignees, value) {
						matchesType = true
						break
					}
				}
			}
			if !matchesType {
				return false
			}
		}
	}

	if strings.TrimSpace(p.search) != "" {
		search := strings.ToLower(p.search)
		if !strings.Contains(strings.ToLower(task.Title), search) &&
			!strings.Contains(strings.ToLower(task.Description), search) &&
			!strings.Contains(strings.ToLower(task.Notes), search) {
			return false
		}
	}
	return true
}

func (p *Program) uniqueValues() ([]string, []string, []string) {
	categories := append([]string{}, p.config.Categories...)
	userMap := map[string]string{}
	for _, user := range p.config.Users {
		id := normalizeUserID(user)
		if _, ok := userMap[id]; !ok {
			userMap[id] = user
		}
	}
	tags := append([]string{}, p.config.Tags...)

	addCategory := func(value string) {
		if value == "" {
			return
		}
		for _, existing := range categories {
			if existing == value {
				return
			}
		}
		categories = append(categories, value)
	}
	addTag := func(value string) {
		value = strings.TrimPrefix(value, "#")
		if value == "" {
			return
		}
		for _, existing := range tags {
			if existing == value {
				return
			}
		}
		tags = append(tags, value)
	}
	addUser := func(value string) {
		id := normalizeUserID(value)
		if id == "" {
			return
		}
		if _, ok := userMap[id]; !ok {
			userMap[id] = value
		}
	}

	for _, task := range p.tasks {
		addCategory(task.Category)
		for _, tag := range task.Tags {
			addTag(tag)
		}
		for _, user := range task.Assignees {
			addUser(user)
		}
	}
	for _, task := range p.archived {
		addCategory(task.Category)
		for _, tag := range task.Tags {
			addTag(tag)
		}
		for _, user := range task.Assignees {
			addUser(user)
		}
	}

	users := make([]string, 0, len(userMap))
	for _, user := range userMap {
		users = append(users, user)
	}
	sort.Strings(users)
	return categories, users, tags
}

func (p *Program) fullUserFormat(userID string) string {
	for _, user := range p.config.Users {
		if normalizeUserID(user) == userID {
			return user
		}
	}
	return userID
}

func parseKanban(content string) (BoardConfig, []Task) {
	config := BoardConfig{}

	configSection := regexp.MustCompile(`(?s)## ⚙️ Configuration\s+(.+?)---`).FindStringSubmatch(content)
	if len(configSection) > 1 {
		configText := configSection[1]
		if match := regexp.MustCompile(`\*\*Columns\*\*:\s*(.+)`).FindStringSubmatch(configText); len(match) > 1 {
			columns := strings.Split(match[1], "|")
			for _, col := range columns {
				parts := regexp.MustCompile(`(.+?)\s*\((.+?)\)`).FindStringSubmatch(strings.TrimSpace(col))
				if len(parts) == 3 {
					config.Columns = append(config.Columns, Column{Name: strings.TrimSpace(parts[1]), ID: strings.TrimSpace(parts[2])})
				}
			}
		}
		if match := regexp.MustCompile(`\*\*Categories\*\*:\s*(.+)`).FindStringSubmatch(configText); len(match) > 1 {
			config.Categories = splitCSV(match[1])
		}
		if match := regexp.MustCompile(`\*\*Users\*\*:\s*(.+)`).FindStringSubmatch(configText); len(match) > 1 {
			config.Users = splitCSV(match[1])
		}
		if match := regexp.MustCompile(`\*\*Tags\*\*:\s*(.+)`).FindStringSubmatch(configText); len(match) > 1 {
			for _, tag := range strings.Fields(match[1]) {
				if strings.HasPrefix(tag, "#") {
					config.Tags = append(config.Tags, strings.TrimPrefix(tag, "#"))
				}
			}
		}
	}

	if len(config.Columns) == 0 {
		config.Columns = defaultColumns()
	}
	if len(config.Categories) == 0 {
		config.Categories = []string{"Frontend", "Backend", "Design", "DevOps", "Tests", "Documentation"}
	}
	if len(config.Users) == 0 {
		config.Users = []string{"@user (User)"}
	}
	if len(config.Tags) == 0 {
		config.Tags = []string{"bug", "feature", "ui", "backend", "urgent", "refactor", "docs", "test"}
	}

	tasks := make([]Task, 0)
	for _, column := range config.Columns {
		tasks = append(tasks, parseTasksFromSection(content, column.Name, column.ID)...)
	}
	return config, tasks
}

func parseArchive(content string) []Task {
	return parseTasksFromSection(content, "✅ Archives", "archived")
}

func parseTasksFromSection(content, sectionName, statusID string) []Task {
	sections := regexp.MustCompile(`\n##\s+`).Split(content, -1)
	var sectionContent string
	for _, section := range sections {
		if strings.HasPrefix(section, sectionName) {
			sectionContent = strings.TrimSpace(strings.TrimPrefix(section, sectionName))
			break
		}
	}
	if sectionContent == "" {
		return nil
	}

	blocks := regexp.MustCompile(`(?m)^###\s+TASK-`).Split(sectionContent, -1)
	if len(blocks) <= 1 {
		return nil
	}

	tasks := make([]Task, 0)
	for _, block := range blocks[1:] {
		lines := strings.Split(block, "\n")
		first := strings.TrimSpace(lines[0])
		pipe := strings.Index(first, "|")
		if pipe <= 0 {
			continue
		}
		idPart := strings.TrimSpace(first[:pipe])
		titlePart := strings.TrimSpace(first[pipe+1:])
		if !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(idPart) {
			continue
		}
		if titlePart == "" {
			continue
		}
		taskContent := strings.Join(lines[1:], "\n")
		task := parseTask("TASK-"+idPart, titlePart, taskContent, statusID)
		tasks = append(tasks, task)
	}
	return tasks
}

func parseTask(id, title, content, status string) Task {
	task := Task{
		ID:        id,
		Title:     strings.TrimSpace(title),
		Status:    status,
		Assignees: []string{},
		Tags:      []string{},
		Subtasks:  []Subtask{},
	}

	metaRe := regexp.MustCompile(`(?m)^\*\*Category\*\*:\s*([^|]+?)(?:\s*\|\s*\*\*Assigned\*\*:\s*(.+?))?$`)
	meta := metaRe.FindStringSubmatch(content)
	if len(meta) == 0 {
		meta = regexp.MustCompile(`(?m)^\*\*Priority\*\*:\s*\w+\s*\|\s*\*\*Category\*\*:\s*([^|]+?)(?:\s*\|\s*\*\*Assigned\*\*:\s*(.+?))?$`).FindStringSubmatch(content)
	}
	if len(meta) > 1 {
		task.Category = strings.TrimSpace(meta[1])
		if len(meta) > 2 && strings.TrimSpace(meta[2]) != "" {
			for _, assignee := range strings.Split(meta[2], ",") {
				trimmed := strings.TrimSpace(assignee)
				if trimmed != "" {
					task.Assignees = append(task.Assignees, trimmed)
				}
			}
		}
	}
	// Allow Assigned to live on its own line (no Category present).
	if len(task.Assignees) == 0 {
		if match := regexp.MustCompile(`(?m)^\*\*Assigned\*\*:\s*(.+)$`).FindStringSubmatch(content); len(match) > 1 {
			for _, assignee := range strings.Split(match[1], ",") {
				trimmed := strings.TrimSpace(assignee)
				if trimmed != "" {
					task.Assignees = append(task.Assignees, trimmed)
				}
			}
		}
	}

	if match := regexp.MustCompile(`\*\*Created\*\*:\s*([\d-]+)`).FindStringSubmatch(content); len(match) > 1 {
		task.Created = match[1]
	}
	if match := regexp.MustCompile(`\*\*Modified\*\*:\s*([\d-]+)`).FindStringSubmatch(content); len(match) > 1 {
		task.Modified = match[1]
	}
	if match := regexp.MustCompile(`\*\*Finished\*\*:\s*([\d-]+)`).FindStringSubmatch(content); len(match) > 1 {
		task.Completed = match[1]
	}
	if match := regexp.MustCompile(`\*\*Tags\*\*:\s*(.+)`).FindStringSubmatch(content); len(match) > 1 {
		for _, tag := range regexp.MustCompile(`#[-\w]+`).FindAllString(match[1], -1) {
			task.Tags = append(task.Tags, tag)
		}
	}

	lines := strings.Split(content, "\n")
	descLines := []string{}
	inDesc := false
	metaLine := regexp.MustCompile(`^\*\*(Priority|Category|Assigned|Created|Modified|Finished|Tags)\*\*`)
	sectionLine := regexp.MustCompile(`^\*\*(Subtasks|Notes|Links|Review|Dependencies)\*\*`)
	for _, line := range lines {
		if metaLine.MatchString(line) {
			continue
		}
		if sectionLine.MatchString(line) {
			break
		}
		if !inDesc {
			if strings.TrimSpace(line) == "" {
				continue
			}
			inDesc = true
		}
		if inDesc {
			descLines = append(descLines, line)
		}
	}
	task.Description = strings.TrimRight(strings.Join(descLines, "\n"), " \n")

	for _, match := range regexp.MustCompile(`(?m)^- \[(x| )\] (.+?)(?: \(due (\d{4}-\d{2}-\d{2})\))?\s*$`).FindAllStringSubmatch(content, -1) {
		completed := match[1] == "x"
		text := strings.TrimSpace(match[2])
		due := ""
		if len(match) > 3 {
			due = normalizeDueDate(match[3])
		}
		task.Subtasks = append(task.Subtasks, Subtask{Completed: completed, Text: text, DueDate: due})
	}

	if idx := strings.Index(content, "**Notes**:"); idx >= 0 {
		after := strings.TrimLeft(content[idx+len("**Notes**:"):], " \n")
		task.Notes = strings.TrimRight(after, "\n")
	}

	return task
}

func (p *Program) generateKanbanMarkdown() string {
	config := p.config
	if len(config.Columns) == 0 {
		config.Columns = defaultColumns()
	}

	categories := append([]string{}, config.Categories...)
	users := append([]string{}, config.Users...)
	tags := append([]string{}, config.Tags...)

	addUnique := func(slice []string, value string) []string {
		for _, existing := range slice {
			if existing == value {
				return slice
			}
		}
		return append(slice, value)
	}

	for _, task := range p.tasks {
		if task.Category != "" {
			categories = addUnique(categories, task.Category)
		}
		for _, user := range task.Assignees {
			if user != "" {
				users = addUnique(users, user)
			}
		}
		for _, tag := range task.Tags {
			clean := strings.TrimPrefix(tag, "#")
			if clean != "" {
				tags = addUnique(tags, clean)
			}
		}
	}

	if len(categories) == 0 {
		categories = []string{"Frontend", "Backend", "Design", "DevOps", "Tests", "Documentation"}
	}
	if len(users) == 0 {
		users = []string{"@user (User)"}
	}
	if len(tags) == 0 {
		tags = []string{"bug", "feature", "ui", "backend", "urgent", "refactor", "docs", "test"}
	}

	var sb strings.Builder
	sb.WriteString("# Kanban Board\n\n")
	sb.WriteString("## ⚙️ Configuration\n\n")
	sb.WriteString("**Columns**: ")
	for i, col := range config.Columns {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(col.Name + " (" + col.ID + ")")
	}
	sb.WriteString("\n\n")
	sb.WriteString("**Categories**: " + strings.Join(categories, ", ") + "\n\n")
	sb.WriteString("**Users**: " + strings.Join(users, ", ") + "\n\n")
	sb.WriteString("**Tags**: " + joinTags(tags) + "\n\n")
	sb.WriteString("---\n\n")

	for _, col := range config.Columns {
		sb.WriteString("## " + col.Name + "\n\n")
		for _, task := range p.tasks {
			if task.Status != col.ID {
				continue
			}
			sb.WriteString("### " + task.ID + " | " + task.Title + "\n")
			meta := ""
			if task.Category != "" {
				meta += "**Category**: " + task.Category
			}
			if len(task.Assignees) > 0 {
				if meta != "" {
					meta += " | "
				}
				meta += "**Assigned**: " + strings.Join(task.Assignees, ", ")
			}
			if meta != "" {
				sb.WriteString(meta + "\n")
			}
			dates := ""
			if task.Created != "" {
				dates += "**Created**: " + task.Created
			}
			if task.Modified != "" {
				if dates != "" {
					dates += " | "
				}
				dates += "**Modified**: " + task.Modified
			}
			if task.Completed != "" {
				if dates != "" {
					dates += " | "
				}
				dates += "**Finished**: " + task.Completed
			}
			if dates != "" {
				sb.WriteString(dates + "\n")
			}
			if len(task.Tags) > 0 {
				sb.WriteString("**Tags**: " + strings.Join(task.Tags, " ") + "\n")
			}
			if task.Description != "" {
				sb.WriteString("\n" + task.Description + "\n")
			}
			if len(task.Subtasks) > 0 {
				sb.WriteString("\n**Subtasks**:\n")
				for _, st := range task.Subtasks {
					due := ""
					if st.DueDate != "" {
						due = " (due " + st.DueDate + ")"
					}
					check := " "
					if st.Completed {
						check = "x"
					}
					sb.WriteString(fmt.Sprintf("- [%s] %s%s\n", check, st.Text, due))
				}
			}
			if task.Notes != "" {
				sb.WriteString("\n**Notes**:\n" + task.Notes + "\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (p *Program) generateArchiveMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# Task Archive\n\n")
	sb.WriteString("> Archived tasks\n\n")
	sb.WriteString("## ✅ Archives\n\n")
	for _, task := range p.archived {
		sb.WriteString("### " + task.ID + " | " + task.Title + "\n")
		meta := ""
		if task.Category != "" {
			meta += "**Category**: " + task.Category
		}
		if len(task.Assignees) > 0 {
			if meta != "" {
				meta += " | "
			}
			meta += "**Assigned**: " + strings.Join(task.Assignees, ", ")
		}
		if meta != "" {
			sb.WriteString(meta + "\n")
		}
		dates := ""
		if task.Created != "" {
			dates += "**Created**: " + task.Created
		}
		if task.Modified != "" {
			if dates != "" {
				dates += " | "
			}
			dates += "**Modified**: " + task.Modified
		}
		if task.Completed != "" {
			if dates != "" {
				dates += " | "
			}
			dates += "**Finished**: " + task.Completed
		}
		if dates != "" {
			sb.WriteString(dates + "\n")
		}
		if len(task.Tags) > 0 {
			sb.WriteString("**Tags**: " + strings.Join(task.Tags, " ") + "\n")
		}
		if task.Description != "" {
			sb.WriteString("\n" + task.Description + "\n")
		}
		if len(task.Subtasks) > 0 {
			sb.WriteString("\n**Subtasks**:\n")
			for _, st := range task.Subtasks {
				due := ""
				if st.DueDate != "" {
					due = " (due " + st.DueDate + ")"
				}
				check := " "
				if st.Completed {
					check = "x"
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s%s\n", check, st.Text, due))
			}
		}
		if task.Notes != "" {
			sb.WriteString("\n**Notes**:\n" + task.Notes + "\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func defaultColumns() []Column {
	return []Column{
		{Name: "📝 To Do", ID: "todo"},
		{Name: "🚀 In Progress", ID: "in-progress"},
		{Name: "👀 In Review", ID: "in-review"},
		{Name: "✅ Done", ID: "done"},
	}
}

func createDefaultKanbanContent() string {
	return "# Kanban Board\n\n## ⚙️ Configuration\n\n**Columns**: 📝 To Do (todo) | 🚀 In Progress (in-progress) | 👀 In Review (in-review) | ✅ Done (done)\n\n**Categories**: Frontend, Backend, Design, DevOps, Tests, Documentation\n\n**Users**: @user (User)\n\n**Tags**: #bug #feature #ui #backend #urgent #refactor #docs #test\n\n---\n\n## 📝 To Do\n\n## 🚀 In Progress\n\n## 👀 In Review\n\n## ✅ Done\n"
}

func createDefaultArchiveContent() string {
	return "# Task Archive\n\n> Archived tasks\n\n## ✅ Archives\n\n"
}

func splitRepo(value string) (string, string, string, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "github.com/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid repo format (expected owner/name)")
	}
	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return "", "", "", fmt.Errorf("invalid repo format (expected owner/name)")
	}
	return owner, name, owner + "/" + name, nil
}

func generateTaskID() string {
	ts := strings.ToUpper(strconvBase36(time.Now().UnixMilli()))
	randPart := strings.ToUpper(strconvBase36(int64(idRand.Intn(1296))))
	if len(randPart) == 1 {
		randPart = "0" + randPart
	}
	return "TASK-" + ts + randPart
}

func strconvBase36(value int64) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var out []byte
	for value > 0 {
		remainder := value % 36
		out = append(out, digits[remainder])
		value /= 36
	}
	if negative {
		out = append(out, '-')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

func normalizeUserID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexAny(value, " ("); idx > 0 {
		return value[:idx]
	}
	return value
}

func hasAssignee(assignees []string, value string) bool {
	for _, assignee := range assignees {
		if normalizeUserID(assignee) == value {
			return true
		}
	}
	return false
}

func hasTag(tags []string, value string) bool {
	for _, tag := range tags {
		if tag == value || tag == "#"+value {
			return true
		}
	}
	return false
}

func parseAssignees(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseTags(value string) []string {
	fields := strings.Fields(value)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		if !strings.HasPrefix(field, "#") {
			field = "#" + field
		}
		out = append(out, field)
	}
	return out
}

func joinTags(tags []string) string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.HasPrefix(tag, "#") {
			out = append(out, tag)
		} else {
			out = append(out, "#"+tag)
		}
	}
	return strings.Join(out, " ")
}

func normalizeDueDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(trimmed) {
		return trimmed
	}
	return ""
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func toMarkupChildren(items []masc.ComponentOrHTML) []masc.MarkupOrChild {
	out := make([]masc.MarkupOrChild, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func logoutCmd() masc.Cmd {
	return func() masc.Msg {
		options := map[string]interface{}{"method": "POST"}
		_, _ = awaitPromise(js.Global().Call("fetch", "/logout", js.ValueOf(options)))
		js.Global().Get("location").Call("reload")
		return nil
	}
}

func clearSessionCmd() masc.Cmd {
	return func() masc.Msg {
		options := map[string]interface{}{"method": "POST"}
		_, _ = awaitPromise(js.Global().Call("fetch", "/logout", js.ValueOf(options)))
		return nil
	}
}

func batchCmds(cmds ...masc.Cmd) masc.Cmd {
	filtered := make([]masc.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			filtered = append(filtered, cmd)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return masc.Batch(filtered...)
}
