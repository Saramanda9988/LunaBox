package main

import (
	"flag"
	"fmt"
	"os"

	"lunabox/internal/utils/updateutils"
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: LunaBoxUpdater.exe <prepare|commit> --task <path>"))
	}

	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	taskPath := flags.String("task", "", "path to a LunaBox update task")
	if err := flags.Parse(os.Args[2:]); err != nil {
		fail(err)
	}
	if *taskPath == "" {
		fail(fmt.Errorf("--task is required"))
	}

	task, err := updateutils.LoadTask(*taskPath)
	if err != nil {
		fail(err)
	}

	switch command {
	case "prepare":
		err = updateutils.Prepare(task)
	case "commit":
		err = updateutils.Commit(task)
		if err != nil {
			_ = updateutils.WriteResult(task, false, err.Error())
		}
		if updateutils.ShouldRestartAfterCommit(err) {
			restartErr := updateutils.Restart(task)
			if err == nil {
				err = restartErr
				if restartErr != nil {
					_ = updateutils.WriteResult(task, false, restartErr.Error())
				}
			} else if restartErr != nil {
				err = fmt.Errorf("%w; restart failed: %v", err, restartErr)
			}
		}
	default:
		err = fmt.Errorf("unknown updater command: %s", command)
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "LunaBox updater:", err)
	os.Exit(1)
}
