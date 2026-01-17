# Markdown CRM

A browser-based Kanban board that uses markdown files for storage. Track leads, manage sales pipelines, and keep your data in version control.

Based on [MarkdownTaskManager](https://github.com/ioniks/MarkdownTaskManager).

## Getting Started

1. **Clone this repository**
   ```bash
   git clone git@github.com:octoberswimmer/crm.git
   cd crm
   ```

2. **Open the application**

   Open `task-manager.html` in a modern browser (Chrome, Edge, or another Chromium-based browser that supports the File System Access API).

3. **Select your project directory**

   Click the "Get Started" button and select the directory containing your `kanban.md` file.

4. **Start managing tasks**

   - Drag and drop cards between columns
   - Click cards to view details
   - Use the "+ Task" button to create new tasks
   - Changes are automatically saved to `kanban.md`

5. **Commit your changes**

   Since tasks are stored in markdown files, you can track changes with git:
   ```bash
   git add kanban.md
   git commit
   ```

## Features

- Drag-and-drop Kanban board
- Customizable columns, categories, and tags
- Subtask management
- Task filtering by tags, category, and assignee
- Automatic last-modified date tracking
- Archive completed tasks
- Multi-language support (English/French)

## File Structure

- `kanban.md` - Active tasks organized by column
- `archive.md` - Archived tasks (created automatically)
- `task-manager.html` - The application (single HTML file)

## Configuration

Edit the Configuration section in `kanban.md` to customize:

```markdown
## ⚙️ Configuration

**Columns**: 📝 Prospect (prospect) | 🚀 Contacted (contacted) | 👨 Dev Trial (dev-trial) | 🏢Company Trial (company-trial) | 💵 Sold (sold) | 👎No Sale (no-sale)

**Categories**: ISV, Consulting Firm, Internal, Independent Consultant

**Users**: @xn, @jim

**Tags**: #packaging #multiple-packages #unpackaged #github #circleci #jenkins #ado #bug-reporter
```

## Browser Requirements

This application uses the File System Access API, which requires:
- Chrome 86+
- Edge 86+
- Opera 72+

Firefox and Safari are not currently supported.
