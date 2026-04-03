//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"syscall/js"
)

const githubRestEndpoint = "https://api.github.com"

type restClient struct {
	Token string
}

func (c *restClient) Get(path string, out interface{}) error {
	url := githubRestEndpoint + path
	headers := map[string]interface{}{
		"Accept": "application/vnd.github+json",
	}
	if c.Token != "" {
		headers["Authorization"] = "Bearer " + c.Token
	}
	options := map[string]interface{}{
		"method":  "GET",
		"headers": headers,
	}
	respValue, err := awaitPromise(js.Global().Call("fetch", url, js.ValueOf(options)))
	if err != nil {
		return err
	}
	ok := respValue.Get("ok").Bool()
	textValue, err := awaitPromise(respValue.Call("text"))
	if err != nil {
		return err
	}
	text := textValue.String()
	if !ok {
		status := respValue.Get("status").Int()
		message := fmt.Sprintf("github api status %d: %s", status, strings.TrimSpace(text))
		if status == 401 {
			return newUnauthorizedError(message)
		}
		return fmt.Errorf("%s", message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(text), out)
}

type userInstallationsResponse struct {
	Installations []userInstallation `json:"installations"`
}

type userInstallation struct {
	ID int64 `json:"id"`
}

type installationRepositoriesResponse struct {
	Repositories []installationRepository `json:"repositories"`
}

type installationRepository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func fetchAccessibleRepos(token string) ([]RepoOption, error) {
	client := &restClient{Token: token}
	installations, err := fetchUserInstallations(client)
	if err != nil {
		return nil, err
	}
	repoMap := map[string]RepoOption{}
	for _, inst := range installations {
		repos, err := fetchInstallationRepos(client, inst.ID)
		if err != nil {
			return nil, err
		}
		for _, repo := range repos {
			full := repo.FullName
			if full == "" && repo.Owner.Login != "" && repo.Name != "" {
				full = repo.Owner.Login + "/" + repo.Name
			}
			if full == "" {
				continue
			}
			repoMap[full] = RepoOption{
				Owner:    repo.Owner.Login,
				Name:     repo.Name,
				FullName: full,
			}
		}
	}
	options := make([]RepoOption, 0, len(repoMap))
	for _, repo := range repoMap {
		options = append(options, repo)
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].FullName < options[j].FullName
	})
	return options, nil
}

func fetchUserInstallations(client *restClient) ([]userInstallation, error) {
	installations := make([]userInstallation, 0)
	page := 1
	for {
		var resp userInstallationsResponse
		path := fmt.Sprintf("/user/installations?per_page=100&page=%d", page)
		if err := client.Get(path, &resp); err != nil {
			return nil, err
		}
		if len(resp.Installations) == 0 {
			break
		}
		installations = append(installations, resp.Installations...)
		if len(resp.Installations) < 100 {
			break
		}
		page++
	}
	return installations, nil
}

func fetchInstallationRepos(client *restClient, installationID int64) ([]installationRepository, error) {
	repos := make([]installationRepository, 0)
	page := 1
	for {
		var resp installationRepositoriesResponse
		path := fmt.Sprintf("/user/installations/%d/repositories?per_page=100&page=%d", installationID, page)
		if err := client.Get(path, &resp); err != nil {
			return nil, err
		}
		if len(resp.Repositories) == 0 {
			break
		}
		repos = append(repos, resp.Repositories...)
		if len(resp.Repositories) < 100 {
			break
		}
		page++
	}
	return repos, nil
}
