package cli

import (
	"context"
	"fmt"
	"os"

	"localci/internal/localci"
)

func (a App) runStart(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("start takes no arguments")
	}

	manager, err := a.newDaemonManager()
	if err != nil {
		return err
	}

	result, err := manager.Start(context.Background())
	if err != nil {
		return err
	}

	if result.Started {
		fmt.Fprintf(a.Stdout, "Started daemon pid %d\nLog: %s\n", result.State.PID, manager.Paths.DaemonLogPath())
		return nil
	}

	fmt.Fprintf(a.Stdout, "Daemon already running with pid %d\nLog: %s\n", result.State.PID, manager.Paths.DaemonLogPath())
	return nil
}

func (a App) runRestart(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("restart takes no arguments")
	}

	manager, err := a.newDaemonManager()
	if err != nil {
		return err
	}

	result, err := manager.Restart(context.Background())
	if err != nil {
		return err
	}

	fmt.Fprintf(a.Stdout, "Restarted daemon pid %d\nLog: %s\n", result.State.PID, manager.Paths.DaemonLogPath())
	return nil
}

func (a App) runStop(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("stop takes no arguments")
	}

	manager, err := a.newDaemonManager()
	if err != nil {
		return err
	}

	if err := manager.Stop(); err != nil {
		return err
	}

	fmt.Fprintf(a.Stdout, "Stopped daemon\nLog: %s\n", manager.Paths.DaemonLogPath())
	return nil
}

func (a App) newDaemonManager() (localci.DaemonManager, error) {
	runner, err := a.newRunner()
	if err != nil {
		return localci.DaemonManager{}, err
	}

	queue := localci.QueueStore{
		Paths: runner.Paths,
	}

	return localci.DaemonManager{
		Paths: runner.Paths,
		Env:   os.Environ(),
		Scheduler: localci.Scheduler{
			Queue:  queue,
			Runner: runner,
		},
	}, nil
}

func (a App) runDaemon(args []string) error {
	if len(args) != 1 || args[0] != "run" {
		return fmt.Errorf("usage: localci daemon run")
	}

	manager, err := a.newDaemonManager()
	if err != nil {
		return err
	}

	ctx, cancel := localci.DaemonContext()
	defer cancel()

	return manager.Run(ctx)
}
