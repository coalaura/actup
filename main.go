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
	File  string
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
			&cli.StringFlag{
				Name:        "file",
				Aliases:     []string{"f"},
				Usage:       "check a specific workflow file instead of .github/workflows",
				Destination: &options.File,
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

	workflows, err := ReadWorkflows(options.File)
	if err != nil {
		return fmt.Errorf("read workflows: %w", err)
	}

	latest, err := FetchLatestReleases(ctx, workflows)
	if err != nil {
		return fmt.Errorf("fetch latest releases: %w", err)
	}

	var (
		changeCount          int
		changedWorkflowCount int
	)

	for workflowIndex := range workflows {
		workflow := &workflows[workflowIndex]

		changes := workflow.Changes(latest, options.Full)

		if options.Apply && len(changes) > 0 {
			err = workflow.Apply(changes)
			if err != nil {
				return fmt.Errorf("update %s: %w", workflow.Path, err)
			}
		}

		err = writeWorkflow(workflow, changes, options.Apply)
		if err != nil {
			return err
		}

		if len(changes) == 0 {
			continue
		}

		changeCount += len(changes)
		changedWorkflowCount++
	}

	if changeCount == 0 {
		return log.Successln("all actions are up to date")
	}

	if options.Apply {
		return log.Successf(
			"updated %d %s in %d %s\n",
			changeCount,
			plural(changeCount, "action", "actions"),
			changedWorkflowCount,
			plural(changedWorkflowCount, "workflow", "workflows"),
		)
	}

	return log.Infof("%d %s available; run with --apply to update\n", changeCount, plural(changeCount, "update", "updates"))
}

func writeWorkflow(workflow *Workflow, changes []Change, applied bool) error {
	err := log.Stepln(workflow.Path)
	if err != nil {
		return fmt.Errorf("write workflow: %w", err)
	}

	var changeIndex int

	for actionIndex := range workflow.Actions {
		action := &workflow.Actions[actionIndex]

		if changeIndex >= len(changes) || changes[changeIndex].Start != action.Start {
			err = log.Subf("%s@%s\n", action.Name, action.Version)
			if err != nil {
				return fmt.Errorf("write action: %w", err)
			}

			continue
		}

		change := &changes[changeIndex]

		changeIndex++

		color := minimal.AnsiInfo

		if applied {
			color = minimal.AnsiSuccess
		}

		err = log.Subf("%s%s@%s -> %s\n", color, action.Name, action.Version, change.Latest)
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
