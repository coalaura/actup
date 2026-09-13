package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/coalaura/plain/minimal"
	"github.com/urfave/cli/v3"
)

type Options struct {
	Apply bool
	Full  bool
}

var Version = "dev"

var log = minimal.New()

func main() {
	err := NewCLI().Run(context.Background(), os.Args)
	if err == nil {
		return
	}

	status, ok := errors.AsType[cli.ExitCoder](err)
	if ok {
		if status.Error() != "" {
			_ = log.Errorln(status.Error())
		}

		os.Exit(status.ExitCode())
	}

	_ = log.Errorln(err.Error())

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

			return Run(ctx, options)
		},
	}
}

func Run(ctx context.Context, options Options) error {
	err := log.Infoln("checking workflow actions")
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
		return log.Successln("all actions are up to date")
	}

	for _, workflowChanges := range changedWorkflows {
		if options.Apply {
			err = workflowChanges.Workflow.Apply(workflowChanges.Changes)
			if err != nil {
				return fmt.Errorf("update %s: %w", workflowChanges.Workflow.Path, err)
			}
		}

		err = writeChanges(workflowChanges)
		if err != nil {
			return err
		}
	}

	if options.Apply {
		return log.Successf(
			"updated %d %s in %d %s\n",
			changeCount,
			plural(changeCount, "action", "actions"),
			len(changedWorkflows),
			plural(len(changedWorkflows), "workflow", "workflows"),
		)
	}

	return log.Infof("%d %s available; run with --apply to update\n", changeCount, plural(changeCount, "update", "updates"))
}

func writeChanges(workflowChanges WorkflowChanges) error {
	err := log.Writeln(minimal.AnsiInfo, " "+workflowChanges.Workflow.Path)
	if err != nil {
		return fmt.Errorf("write workflow: %w", err)
	}

	for _, change := range workflowChanges.Changes {
		err = log.Subf("%s %s -> %s\n", change.Name, change.Current, change.Latest)
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
