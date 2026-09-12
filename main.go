package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"
)

const (
	colorCyan  = "\x1b[36m"
	colorGreen = "\x1b[32m"
	colorRed   = "\x1b[31m"
	colorGray  = "\x1b[90m"
	colorReset = "\x1b[0m"
)

type Options struct {
	Apply bool
	Full  bool
}

var Version = "dev"

func main() {
	err := NewCLI().Run(context.Background(), os.Args)
	if err == nil {
		return
	}

	status, ok := errors.AsType[cli.ExitCoder](err)
	if ok {
		if status.Error() != "" {
			writeError(os.Stderr, status.Error())
		}

		os.Exit(status.ExitCode())
	}

	writeError(os.Stderr, err.Error())

	os.Exit(1)
}

func NewCLI() *cli.Command {
	var options Options

	return &cli.Command{
		Name:                   "actup",
		Usage:                  "update GitHub Actions in workflow files",
		UsageText:              "actup [options]",
		Version:                Version,
		UseShortOptionHandling: true,
		ExitErrHandler:         func(context.Context, *cli.Command, error) {},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "apply",
				Usage:       "write available updates to workflow files",
				Destination: &options.Apply,
			},
			&cli.BoolFlag{
				Name:        "full",
				Usage:       "include minor and patch version updates",
				Destination: &options.Full,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 0 {
				return fmt.Errorf("unexpected argument: %s", cmd.Args().First())
			}

			return Run(ctx, cmd.Writer, options)
		},
	}
}

func Run(ctx context.Context, writer io.Writer, options Options) error {
	err := writeInfo(writer, "checking workflow actions")
	if err != nil {
		return err
	}

	workflows, err := ReadWorkflows()
	if err != nil {
		return fmt.Errorf("read workflows: %w", err)
	}

	latest, err := FetchLatestReleases(ctx, workflows)
	if err != nil {
		return fmt.Errorf("fetch latest releases: %w", err)
	}

	changedWorkflows := make([]WorkflowChanges, 0, len(workflows))
	changeCount := 0

	for workflowIndex := range workflows {
		workflow := &workflows[workflowIndex]

		changes := workflow.Changes(latest, options.Full)
		if len(changes) == 0 {
			continue
		}

		changedWorkflows = append(changedWorkflows, WorkflowChanges{
			Workflow: workflow,
			Changes:  changes,
		})

		changeCount += len(changes)
	}

	if changeCount == 0 {
		return writeSuccess(writer, "all actions are up to date")
	}

	for _, workflowChanges := range changedWorkflows {
		if options.Apply {
			err = workflowChanges.Workflow.Apply(workflowChanges.Changes)
			if err != nil {
				return fmt.Errorf("update %s: %w", workflowChanges.Workflow.Path, err)
			}
		}

		err = writeChanges(writer, workflowChanges)
		if err != nil {
			return err
		}
	}

	if options.Apply {
		return writeSuccess(
			writer,
			"updated %d %s in %d %s",
			changeCount,
			plural(changeCount, "action", "actions"),
			len(changedWorkflows),
			plural(len(changedWorkflows), "workflow", "workflows"),
		)
	}

	return writeInfo(writer, "%d %s available; run with --apply to update", changeCount, plural(changeCount, "update", "updates"))
}

func writeInfo(writer io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(writer, colorCyan+"::"+colorReset+" "+format+"\n", args...)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func writeSuccess(writer io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(writer, colorGreen+"::"+colorReset+" "+format+"\n", args...)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func writeError(writer io.Writer, message string) {
	fmt.Fprintf(writer, colorRed+"!!"+colorReset+" %s\n", message)
}

func writeChanges(writer io.Writer, workflowChanges WorkflowChanges) error {
	_, err := fmt.Fprintf(writer, " %s%s%s\n", colorCyan, workflowChanges.Workflow.Path, colorReset)
	if err != nil {
		return fmt.Errorf("write workflow: %w", err)
	}

	for _, change := range workflowChanges.Changes {
		_, err = fmt.Fprintf(
			writer,
			"   %s-> %s %s -> %s%s\n",
			colorGray,
			change.Name,
			change.Current,
			change.Latest,
			colorReset,
		)

		if err != nil {
			return fmt.Errorf("write action update: %w", err)
		}
	}

	return nil
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}

	return plural
}
