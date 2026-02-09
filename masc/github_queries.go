//go:build js && wasm

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

func fetchViewer(client *GraphQLClient) (User, error) {
	query := `query { viewer { id login avatarUrl } }`
	var resp struct {
		Viewer struct {
			ID        string `json:"id"`
			Login     string `json:"login"`
			AvatarURL string `json:"avatarUrl"`
		} `json:"viewer"`
	}
	if err := client.Query(query, nil, &resp); err != nil {
		return User{}, err
	}
	return User{ID: resp.Viewer.ID, Login: resp.Viewer.Login, AvatarURL: resp.Viewer.AvatarURL}, nil
}

type repoFiles struct {
	Branch         string
	HeadOID        string
	KanbanContent  string
	ArchiveContent string
	MissingKanban  bool
	MissingArchive bool
}

func fetchRepoFiles(client *GraphQLClient, owner, name, kanbanPath, archivePath string) (repoFiles, error) {
	if owner == "" || name == "" {
		return repoFiles{}, errors.New("repository is required")
	}
	kanbanExpr := fmt.Sprintf("HEAD:%s", strings.TrimPrefix(kanbanPath, "/"))
	archiveExpr := fmt.Sprintf("HEAD:%s", strings.TrimPrefix(archivePath, "/"))
	query := `query($owner: String!, $name: String!, $kanbanExpr: String!, $archiveExpr: String!) {
		repository(owner: $owner, name: $name) {
			defaultBranchRef { name target { oid } }
			kanban: object(expression: $kanbanExpr) { ... on Blob { text oid } }
			archive: object(expression: $archiveExpr) { ... on Blob { text oid } }
		}
	}`
	vars := map[string]interface{}{
		"owner":       owner,
		"name":        name,
		"kanbanExpr":  kanbanExpr,
		"archiveExpr": archiveExpr,
	}
	var resp struct {
		Repository struct {
			DefaultBranchRef struct {
				Name   string `json:"name"`
				Target struct {
					OID string `json:"oid"`
				} `json:"target"`
			} `json:"defaultBranchRef"`
			Kanban struct {
				Text string `json:"text"`
				OID  string `json:"oid"`
			} `json:"kanban"`
			Archive struct {
				Text string `json:"text"`
				OID  string `json:"oid"`
			} `json:"archive"`
		} `json:"repository"`
	}
	if err := client.Query(query, vars, &resp); err != nil {
		return repoFiles{}, err
	}
	branch := resp.Repository.DefaultBranchRef.Name
	if branch == "" {
		branch = "main"
	}
	headOID := resp.Repository.DefaultBranchRef.Target.OID
	kanbanMissing := resp.Repository.Kanban.OID == ""
	archiveMissing := resp.Repository.Archive.OID == ""

	return repoFiles{
		Branch:         branch,
		HeadOID:        headOID,
		KanbanContent:  resp.Repository.Kanban.Text,
		ArchiveContent: resp.Repository.Archive.Text,
		MissingKanban:  kanbanMissing,
		MissingArchive: archiveMissing,
	}, nil
}

type commitResult struct {
	URL string
	OID string
}

func commitRepoFiles(client *GraphQLClient, repoName, branch, headOID, message string, files map[string]string) (commitResult, error) {
	if repoName == "" || branch == "" {
		return commitResult{}, errors.New("repo and branch are required")
	}
	if len(files) == 0 {
		return commitResult{}, errors.New("no files to commit")
	}

	// For empty repos (no headOID), use REST API to create initial commit
	if headOID == "" {
		return commitRepoFilesREST(client, repoName, branch, message, files)
	}

	return commitRepoFilesGraphQL(client, repoName, branch, headOID, message, files)
}

func commitRepoFilesREST(client *GraphQLClient, repoName, branch, message string, files map[string]string) (commitResult, error) {
	baseURL := fmt.Sprintf("https://api.github.com/repos/%s", repoName)

	// For empty repos, we must use Contents API to create the first file,
	// then use GraphQL to add all files in a second commit
	var firstPath string
	for path := range files {
		firstPath = path
		break
	}
	firstPath = strings.TrimPrefix(firstPath, "/")
	firstContent := files["/"+firstPath]
	if firstContent == "" {
		firstContent = files[firstPath]
	}

	// Create first file to initialize the repo
	encoded := base64.StdEncoding.EncodeToString([]byte(firstContent))
	resp, err := client.REST("PUT", baseURL+"/contents/"+firstPath, map[string]interface{}{
		"message": "Initialize repository",
		"content": encoded,
		"branch":  branch,
	})
	if err != nil {
		return commitResult{}, fmt.Errorf("init repo: %w", err)
	}

	// Get the new HEAD OID
	commit, _ := resp["commit"].(map[string]interface{})
	headOID, _ := commit["sha"].(string)
	if headOID == "" {
		return commitResult{}, errors.New("failed to get commit SHA after init")
	}

	// Now use GraphQL to commit all files
	return commitRepoFilesGraphQL(client, repoName, branch, headOID, message, files)
}

func commitRepoFilesGraphQL(client *GraphQLClient, repoName, branch, headOID, message string, files map[string]string) (commitResult, error) {
	additions := make([]map[string]interface{}, 0, len(files))
	for path, content := range files {
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		additions = append(additions, map[string]interface{}{
			"path":     strings.TrimPrefix(path, "/"),
			"contents": encoded,
		})
	}

	query := `mutation($input: CreateCommitOnBranchInput!) {
		createCommitOnBranch(input: $input) {
			commit { oid url }
		}
	}`
	input := map[string]interface{}{
		"branch": map[string]interface{}{
			"repositoryNameWithOwner": repoName,
			"branchName":              branch,
		},
		"message": map[string]interface{}{
			"headline": message,
		},
		"expectedHeadOid": headOID,
		"fileChanges": map[string]interface{}{
			"additions": additions,
		},
	}
	vars := map[string]interface{}{"input": input}
	var resp struct {
		CreateCommitOnBranch struct {
			Commit struct {
				OID string `json:"oid"`
				URL string `json:"url"`
			} `json:"commit"`
		} `json:"createCommitOnBranch"`
	}
	if err := client.Query(query, vars, &resp); err != nil {
		return commitResult{}, err
	}
	return commitResult{URL: resp.CreateCommitOnBranch.Commit.URL, OID: resp.CreateCommitOnBranch.Commit.OID}, nil
}
