package main

import (
	"flag"
	"fmt"
	"os"

	"lunabox/updater/updateutils"
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: LunaBoxUpdater.exe <prepare|commit> --task <path>"))
	}

	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	taskPath := flags.String("task", "", "path to a LunaBox update task")
	elevated := flags.Bool("elevated", false, "internal flag: this process already has UAC elevation")
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
		if err != nil {
			// LunaBox reads this marker to report a normalized failure reason.
			if recordErr := updateutils.WritePrepareFailure(task, err); recordErr != nil {
				err = fmt.Errorf("%w; record prepare failure: %v", err, recordErr)
			}
		}
	case "commit":
		err = updateutils.Commit(task)
		if err != nil && !*elevated && updateutils.CanRetryElevated(err) {
			if retryErr := updateutils.StartElevatedCommit(*taskPath, task.WorkDir); retryErr == nil {
				return
			} else {
				err = fmt.Errorf("%w; elevated retry failed: %v", err, retryErr)
			}
		}
		if err != nil {
			_ = updateutils.WriteFailure(task, err)
		}
		if updateutils.ShouldRestartAfterCommit(err) {
			restartErr := updateutils.Restart(task)
			if err == nil {
				err = restartErr
				if restartErr != nil {
					_ = updateutils.WriteFailure(task, restartErr)
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
