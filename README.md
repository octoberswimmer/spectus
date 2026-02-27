# Spectus - Markdown Kanban for Git

A GitHub-backed Kanban board that reads and writes `kanban.md` + `archive.md` directly in a repository using the GitHub GraphQL API. The UI is a Masc (Go WASM) app, and the server handles GitHub OAuth.

Spectus is a port of [MarkdownTaskManager](https://github.com/ioniks/MarkdownTaskManager), an HTML/JavaScript application, to a [masc](https://github.com/octoberswimmer/masc) application.

## Features
- Load a repo + markdown paths from the UI.
- Edit tasks, subtasks, tags, and columns.
- Archive and restore tasks.
- Commit changes directly to the repo (default branch).
- Persist repo settings in localStorage per user.

## Requirements
- Go 1.25+
- A GitHub OAuth App (or GitHub App with OAuth flow) configured for this server.

## Quick Start
1. Build the WASM bundle:
   ```bash
   make build
   ```
2. Run the server:
   ```bash
   go run .
   ```
   or build a binary:
   ```bash
   make kanban
   ./bin/kanban
   ```
3. Visit `http://localhost:8080`, log in with GitHub, enter a repo (owner/name), and load your board.

## Environment Variables
Required:
- `CLIENT_ID`: GitHub OAuth client ID.
- `CLIENT_SECRET`: GitHub OAuth client secret.

Recommended (persist sessions across restarts):
- `HASH_KEY`: 32-byte hash key for secure cookies.
- `BLOCK_KEY`: 32-byte block key for secure cookies.

Suggested generation:
```bash
# 32-byte hash key (32 chars)
HASH_KEY=$(openssl rand -base64 24)

# 32-byte block key (32 chars; AES-256)
BLOCK_KEY=$(openssl rand -base64 24)
```

Optional:
- `ADDR`: Server listen address. Default `:8080`.
- `PORT`: Used if `ADDR` is not set.
- `PUBLIC_URL`: Base URL for OAuth callback and secure cookie flag. Default derived from `ADDR`.
- `GITHUB_SCOPES`: Comma-separated scopes. Default `repo,read:user`.
- `SESSION_COOKIE`: Cookie name. Default `spectus_session`.
- `KANBAN_REPO`: Default repo in `owner/name` form (shown in the UI).
- `KANBAN_PATH`: Default kanban path. Default `kanban.md`.
- `ARCHIVE_PATH`: Default archive path. Default `archive.md`.
- `GITHUB_WEBHOOK_SECRET`: Secret for verifying GitHub webhook signatures (for real-time sync).

## GitHub OAuth Setup
Create a GitHub OAuth App and set:
- Homepage URL: `PUBLIC_URL`
- Authorization callback URL: `PUBLIC_URL/auth/github/callback`

Make sure the app has the scopes listed in `GITHUB_SCOPES` (the default includes `repo`).

## GitHub Webhook Setup (Optional)
To enable real-time sync when other users commit changes:
1. In your GitHub App settings, enable webhooks
2. Set the webhook URL to `PUBLIC_URL/webhook`
3. Set a webhook secret and configure `GITHUB_WEBHOOK_SECRET` to match
4. Subscribe to "Push" events

When another user commits changes to `kanban.md` or `archive.md`, all connected clients will automatically sync.

## File Structure
- `kanban.md` - Active tasks organized by column
- `archive.md` - Archived tasks
- `masc/` - Go WASM UI
- `templates/` - HTML shell
- `static/` - CSS + WASM assets

## Notes
- Commits go to the repo’s default branch using `createCommitOnBranch`.
- If `kanban.md` or `archive.md` is missing, defaults are generated and the board starts in a dirty state until you commit.
