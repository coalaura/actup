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

type Version struct {
	Major string
	Full  string
}

var client = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func FetchLatestReleases(workflows []Workflow, full bool) (map[string]semver.SemVer, error) {
	var (
		wg      sync.WaitGroup
		mx      sync.Mutex
		actions = make(map[string]struct{})
	)

	for _, workflow := range workflows {
		for _, action := range workflow.Actions {
			name := action.Parent

			if _, ok := actions[name]; ok {
				continue
			}

			actions[name] = struct{}{}
		}
	}

	if len(actions) == 0 {
		return make(map[string]semver.SemVer), nil
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	jobs := make(chan string, len(actions))
	results := make(map[string]semver.SemVer, len(actions))

	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}

					latest, err := FetchLatestRelease(job)
					if err != nil {
						cancel(err)

						return
					}

					if !full {
						latest.SetMajorOnly()
					}

					mx.Lock()
					results[job] = latest
					mx.Unlock()
				}
			}
		})
	}

	for action := range actions {
		jobs <- action
	}

	close(jobs)
	wg.Wait()

	err := context.Cause(ctx)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func FetchLatestRelease(name string) (semver.SemVer, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/latest", name)

	resp, err := client.Head(url)
	if err != nil {
		return semver.SemVer{}, err
	}

	resp.Body.Close()

	if resp.StatusCode != 302 {
		return semver.SemVer{}, errors.New(resp.Status)
	}

	target := resp.Header.Get("Location")
	if target == "" {
		return semver.SemVer{}, errors.New("no location header")
	}

	slash := strings.LastIndexByte(target, '/')
	if slash == -1 {
		return semver.SemVer{}, fmt.Errorf("invalid location header: %q", target)
	}

	return semver.ParseSemVer(target[slash+1:], true)
}
