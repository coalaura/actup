package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coalaura/semver"
)

const fetchWorkers = 4

var client = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func FetchLatestReleases(ctx context.Context, workflows []Workflow) (map[string]semver.SemVer, error) {
	repositories := actionRepositories(workflows)

	results := make(map[string]semver.SemVer, len(repositories))

	if len(repositories) == 0 {
		return results, nil
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	jobs := make(chan string, len(repositories))

	var (
		waitGroup sync.WaitGroup
		mutex     sync.Mutex
	)

	workerCount := min(fetchWorkers, len(repositories))

	for range workerCount {
		waitGroup.Go(func() {
			for repository := range jobs {
				if ctx.Err() != nil {
					return
				}

				latest, err := FetchLatestRelease(ctx, repository)
				if err != nil {
					cancel(fmt.Errorf("%s: %w", repository, err))

					return
				}

				mutex.Lock()
				results[repository] = latest
				mutex.Unlock()
			}
		})
	}

	for repository := range repositories {
		jobs <- repository
	}

	close(jobs)
	waitGroup.Wait()

	err := context.Cause(ctx)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func FetchLatestRelease(ctx context.Context, name string) (semver.SemVer, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/latest", name)

	request, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return semver.SemVer{}, fmt.Errorf("create request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return semver.SemVer{}, err
	}

	err = response.Body.Close()
	if err != nil {
		return semver.SemVer{}, fmt.Errorf("close response: %w", err)
	}

	if response.StatusCode < http.StatusMultipleChoices || response.StatusCode >= http.StatusBadRequest {
		return semver.SemVer{}, errors.New(response.Status)
	}

	target := response.Header.Get("Location")
	if target == "" {
		return semver.SemVer{}, errors.New("no location header")
	}

	slash := strings.LastIndexByte(target, '/')
	if slash == -1 || slash == len(target)-1 {
		return semver.SemVer{}, fmt.Errorf("invalid location header: %q", target)
	}

	version, err := semver.ParseSemVer(target[slash+1:], true)
	if err != nil {
		return semver.SemVer{}, fmt.Errorf("parse release version: %w", err)
	}

	return version, nil
}

func actionRepositories(workflows []Workflow) map[string]struct{} {
	repositories := make(map[string]struct{})

	for _, workflow := range workflows {
		for _, action := range workflow.Actions {
			repositories[action.Parent] = struct{}{}
		}
	}

	return repositories
}
