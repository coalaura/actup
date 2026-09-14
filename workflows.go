package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coalaura/semver"
)

type Action struct {
	Name    string
	Parent  string
	Version semver.SemVer
	Start   int
	End     int
}

type Change struct {
	Latest semver.SemVer
	Start  int
	End    int
}

type Workflow struct {
	Path    string
	Data    []byte
	Actions []Action
}

func (workflow *Workflow) Changes(latest map[string]semver.SemVer, full bool) []Change {
	changes := make([]Change, 0, len(workflow.Actions))

	for _, action := range workflow.Actions {
		version, ok := latest[action.Parent]
		if !ok {
			continue
		}

		if !full {
			version.SetMajorOnly()
		}

		if !version.HigherThan(action.Version) {
			continue
		}

		changes = append(changes, Change{
			Latest: version,
			Start:  action.Start,
			End:    action.End,
		})
	}

	return changes
}

func (workflow *Workflow) Apply(changes []Change) error {
	if len(changes) == 0 {
		return nil
	}

	var additional int

	for _, change := range changes {
		additional += len(change.Latest.String()) - (change.End - change.Start)
	}

	var (
		buffer bytes.Buffer
		offset int
	)

	buffer.Grow(len(workflow.Data) + additional)

	for _, change := range changes {
		buffer.Write(workflow.Data[offset:change.Start])
		buffer.WriteString(change.Latest.String())

		offset = change.End
	}

	buffer.Write(workflow.Data[offset:])

	return os.WriteFile(workflow.Path, buffer.Bytes(), 0o644)
}

func ReadWorkflows(file string) ([]Workflow, error) {
	if file != "" {
		if !isYAMLFile(file) {
			return nil, fmt.Errorf("%s: not a YAML workflow file", file)
		}

		workflow, err := readWorkflow(file)
		if err != nil {
			return nil, err
		}

		return []Workflow{workflow}, nil
	}

	directory := filepath.Join(".github", "workflows")

	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	workflows := make([]Workflow, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || !isYAMLFile(name) {
			continue
		}

		workflow, err := readWorkflow(filepath.Join(directory, name))
		if err != nil {
			return nil, err
		}

		workflows = append(workflows, workflow)
	}

	return workflows, nil
}

func readWorkflow(path string) (Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, err
	}

	return Workflow{
		Path:    path,
		Data:    data,
		Actions: findActions(data),
	}, nil
}

func isYAMLFile(name string) bool {
	extension := filepath.Ext(name)

	return strings.EqualFold(extension, ".yaml") || strings.EqualFold(extension, ".yml")
}

func findActions(data []byte) []Action {
	var actions []Action

	for lineStart := 0; lineStart < len(data); {
		lineLength := bytes.IndexByte(data[lineStart:], '\n')
		lineEnd := len(data)

		if lineLength >= 0 {
			lineEnd = lineStart + lineLength + 1
		}

		action, ok := findAction(data[lineStart:lineEnd], lineStart)
		if ok {
			actions = append(actions, action)
		}

		lineStart = lineEnd
	}

	return actions
}

func findAction(data []byte, base int) (Action, bool) {
	var action Action

	trimmed, leading := trimLeftSpace(data)
	position := base + leading

	if len(trimmed) == 0 {
		return action, false
	}

	if trimmed[0] == '-' {
		trimmed, leading = trimLeftSpace(trimmed[1:])
		position += leading + 1
	}

	if !bytes.HasPrefix(trimmed, []byte("uses:")) {
		return action, false
	}

	trimmed, leading = trimLeftSpace(trimmed[len("uses:"):])
	position += leading + len("uses:")

	if len(trimmed) == 0 {
		return action, false
	}

	var quote byte

	if trimmed[0] == '"' || trimmed[0] == '\'' {
		quote = trimmed[0]
		trimmed = trimmed[1:]

		position++
	}

	at := bytes.IndexByte(trimmed, '@')
	if at <= 0 {
		return action, false
	}

	name := bytes.TrimRight(trimmed[:at], " \t")
	version := trimmed[at+1:]

	if quote != 0 {
		end := bytes.IndexByte(version, quote)
		if end >= 0 {
			version = version[:end]
		}
	} else {
		end := bytes.IndexAny(version, " \t\r\n#")
		if end >= 0 {
			version = version[:end]
		}
	}

	if shouldSkipActionName(name) {
		return action, false
	}

	parsed, err := semver.ParseSemVer(string(version), true)
	if err != nil {
		return action, false
	}

	return Action{
		Name:    string(name),
		Parent:  getActionParent(name),
		Version: parsed,
		Start:   position + at + 1,
		End:     position + at + 1 + len(version),
	}, true
}

func trimLeftSpace(data []byte) ([]byte, int) {
	var index int

	for index < len(data) {
		switch data[index] {
		case ' ', '\t', '\r', '\n':
			index++
		default:
			return data[index:], index
		}
	}

	return data[index:], index
}

func shouldSkipActionName(name []byte) bool {
	if len(name) == 0 {
		return true
	}

	return (name[0] < 'a' || name[0] > 'z') && (name[0] < 'A' || name[0] > 'Z')
}

func getActionParent(name []byte) string {
	firstSlash := bytes.IndexByte(name, '/')
	if firstSlash == -1 {
		return string(name)
	}

	secondSlash := bytes.IndexByte(name[firstSlash+1:], '/')
	if secondSlash == -1 {
		return string(name)
	}

	return string(name[:firstSlash+1+secondSlash])
}
