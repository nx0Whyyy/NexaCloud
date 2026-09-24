package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type GitHubClient struct {
	repository string
	token      string
	client     *http.Client
	mu         sync.RWMutex
	cached     changelogResponse
	expiresAt  time.Time
}

type changelogResponse struct {
	Repository string    `json:"repository"`
	UpdatedAt  time.Time `json:"updated_at"`
	Releases   []release `json:"releases"`
	Commits    []commit  `json:"commits"`
}

type release struct {
	Name        string    `json:"name"`
	Tag         string    `json:"tag"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
}

type commit struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	URL     string    `json:"url"`
	Date    time.Time `json:"date"`
}

func NewGitHubClient(repository, token string) *GitHubClient {
	return &GitHubClient{repository: repository, token: token, client: &http.Client{Timeout: 8 * time.Second}}
}

func (g *GitHubClient) Changelog(ctx context.Context) (changelogResponse, error) {
	g.mu.RLock()
	if time.Now().Before(g.expiresAt) {
		cached := g.cached
		g.mu.RUnlock()
		return cached, nil
	}
	g.mu.RUnlock()

	var releasesRaw []struct {
		Name        string    `json:"name"`
		TagName     string    `json:"tag_name"`
		Body        string    `json:"body"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
		Prerelease  bool      `json:"prerelease"`
	}
	var commitsRaw []struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Commit  struct {
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := g.get(ctx, "/releases?per_page=10", &releasesRaw); err != nil {
		return changelogResponse{}, err
	}
	if err := g.get(ctx, "/commits?per_page=30", &commitsRaw); err != nil {
		return changelogResponse{}, err
	}
	result := changelogResponse{Repository: g.repository, UpdatedAt: time.Now().UTC(), Releases: make([]release, 0, len(releasesRaw)), Commits: make([]commit, 0, len(commitsRaw))}
	for _, item := range releasesRaw {
		name := item.Name
		if name == "" {
			name = item.TagName
		}
		result.Releases = append(result.Releases, release{Name: name, Tag: item.TagName, Body: item.Body, URL: item.HTMLURL, PublishedAt: item.PublishedAt, Prerelease: item.Prerelease})
	}
	for _, item := range commitsRaw {
		result.Commits = append(result.Commits, commit{SHA: shortSHA(item.SHA), Message: strings.Split(item.Commit.Message, "\n")[0], Author: item.Commit.Author.Name, URL: item.HTMLURL, Date: item.Commit.Author.Date})
	}
	g.mu.Lock()
	g.cached = result
	g.expiresAt = time.Now().Add(time.Minute)
	g.mu.Unlock()
	return result, nil
}

func (g *GitHubClient) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+g.repository+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "NexaCloud-Monitoring")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	response, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("github returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
