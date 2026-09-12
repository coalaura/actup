package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/coalaura/semver"
)

type Action struct {
	Name    string
	Parent  string
	Version semver.SemVer
	Start   int
	End     int
}

type Workflow struct {
	Path    string
	Data    []byte
	Actions []Action
}

func (w *Workflow) Update(latest map[string]semver.SemVer) error {
	update := make([]Action, 0, len(w.Actions))

	for _, action := range w.Actions {
		version, ok := latest[action.Name]
		if !ok {
			continue
		}

		if !version.HigherThan(action.Version) {
			continue
		}

		update = append(update, action)
	}

	if len(update) == 0 {
		return nil
	}

	var (
		offset int
		next   int
		buf    bytes.Buffer
	)

	buf.Grow(len(w.Data))

	for _, action := range update {
		next = action.Start

		buf.Write(w.Data[offset:next])

		version := latest[action.Name]

		buf.WriteString(version.MajorString())

		offset = action.End
	}

	if offset < len(w.Data) {
		buf.Write(w.Data[offset:])
	}

	return os.WriteFile(w.Path, buf.Bytes(), 0644)
}

func ReadWorkflows() ([]Workflow, error) {
	directory := filepath.Join(".github", "workflows")

	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	workflows := make([]Workflow, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || !isYamlFile(name) {
			continue
		}

		path := filepath.Join(directory, name)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		workflows = append(workflows, Workflow{
			Path:    path,
			Data:    data,
			Actions: findActions(data),
		})
	}

	return workflows, nil
}

func isYamlFile(name string) bool {
	idx := strings.LastIndex(name, ".")
	if idx == -1 || len(name)-idx < 4 {
		return false
	}

	return name[idx:] == ".yaml" || name[idx:] == ".yml"
}

func findActions(data []byte) []Action {
	var (
		index  int
		offset int

		actions []Action
	)

	for {
		offset = bytes.IndexByte(data[index:], '\n')
		if offset == -1 {
			break
		}

		offset += index + 1

		start, line := trimLeftSpace(data[index:offset])
		if len(line) < 6 {
			index = offset

			continue
		}

		index += start

		if line[0] == '-' {
			start, line = trimLeftSpace(line[1:])

			index += start + 1
		}

		if !bytes.HasPrefix(line, []byte("uses:")) {
			index = offset

			continue
		}

		start, line = trimLeftSpace(line[5:])

		index += start + 5

		if line[0] == '"' || line[0] == '\'' {
			line = line[1:]

			index++
		}

		var version []byte

		at := bytes.IndexByte(line, '@')
		if at == -1 {
			line = bytes.TrimRight(line, " \t\r\n\"'")
		} else {
			version = bytes.TrimRight(line[at+1:], " \t\r\n\"'")

			line = line[:at]

			index += at + 1
		}

		if shouldSkipActionName(line) {
			index = offset

			continue
		}

		ver, err := semver.ParseSemVer(asString(version), true)
		if err != nil {
			continue
		}

		actions = append(actions, Action{
			Name:    asString(line),
			Parent:  getActionParent(line),
			Version: ver,
			Start:   index,
			End:     index + len(version),
		})

		index = offset
	}

	return actions
}

func trimLeftSpace(data []byte) (int, []byte) {
	var index int

	for index < len(data) {
		switch data[index] {
		case ' ', '\t', '\r', '\n':
			index++

			continue
		}

		break
	}

	return index, data[index:]
}

func shouldSkipActionName(name []byte) bool {
	if len(name) < 1 {
		return true
	}

	return (name[0] < 'a' || name[0] > 'z') && (name[0] < 'A' || name[0] > 'Z')
}

func getActionParent(name []byte) string {
	slash := bytes.IndexByte(name, '/')
	if slash == -1 {
		return asString(name)
	}

	slash++

	secondary := bytes.IndexByte(name[slash:], '/')
	if secondary == -1 {
		return asString(name)
	}

	return asString(name[:slash+secondary])
}

func asString(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	return unsafe.String(unsafe.SliceData(b), len(b))
}
