package cli

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"localci/internal/localci"
)

func parsePruneAge(value string) (time.Duration, error) {
	if strings.HasSuffix(value, "d") {
		days, err := strconv.ParseInt(strings.TrimSuffix(value, "d"), 10, 64)
		if err != nil || days < 0 || days > math.MaxInt64/int64(24*time.Hour) {
			return 0, fmt.Errorf("invalid age %q", value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 {
		return 0, fmt.Errorf("invalid age %q (use e.g. 30d or 24h)", value)
	}
	return duration, nil
}

func parsePruneSize(value string) (int64, error) {
	for _, unit := range []struct {
		suffix string
		bytes  int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"GB", 1_000_000_000}, {"MB", 1_000_000}, {"KB", 1_000}, {"B", 1}, {"", 1}} {
		if !strings.HasSuffix(value, unit.suffix) {
			continue
		}
		number, err := strconv.ParseFloat(strings.TrimSuffix(value, unit.suffix), 64)
		bytes := number * float64(unit.bytes)
		if err != nil || math.IsNaN(bytes) || math.IsInf(bytes, 0) || bytes < 0 || bytes >= float64(math.MaxInt64) {
			return 0, fmt.Errorf("invalid size %q (use e.g. 100MB, 10GB, or 10GiB)", value)
		}
		return int64(bytes), nil
	}
	return 0, fmt.Errorf("invalid size %q", value)
}

func (a App) newPruneCommand() *cobra.Command {
	var age, size, larger string
	var yes, dryRun bool
	cmd := &cobra.Command{
		Use: "prune", Short: "Remove old or large task outputs", GroupID: managementGroupID, Args: cobra.NoArgs,
		Long:    "Remove logs and artifacts across all repositories. By default, remove outputs older than 30 days, then the oldest remaining outputs until at most 10 GiB remain. Set either limit to 0 to disable it. Sizes accept decimal KB/MB/GB or binary KiB/MiB/GiB. Alternatively, use --larger-than to remove individual files above a size regardless of age. Active and queued runs, task records, and caches are preserved. Nothing is removed until you confirm or pass --yes.",
		Example: "localci prune --dry-run\nlocalci prune\nlocalci prune --max-age 7d --max-size 5GiB --yes\nlocalci prune --larger-than 100MiB",
		RunE: func(cmd *cobra.Command, args []string) error {
			maxAge, err := parsePruneAge(age)
			if err != nil {
				return err
			}
			maxSize, err := parsePruneSize(size)
			if err != nil {
				return err
			}
			largerThan, err := parsePruneSize(larger)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("larger-than") {
				if largerThan == 0 {
					return fmt.Errorf("--larger-than must be greater than zero")
				}
				if cmd.Flags().Changed("max-age") || cmd.Flags().Changed("max-size") {
					return fmt.Errorf("--larger-than cannot be combined with --max-age or --max-size")
				}
			}
			paths, err := a.localCIPaths()
			if err != nil {
				return err
			}
			plan, err := localci.PlanPrune(paths, localci.PruneOptions{MaxAge: maxAge, MaxSize: maxSize, LargerThan: largerThan}, time.Now())
			if err != nil {
				return err
			}
			files := 0
			for _, run := range plan.Runs {
				fmt.Fprintf(a.Stdout, "%s %s: %d files, %s\n", run.RepoDir, run.Commit, len(run.Files), pruneBytes(run.Bytes))
				if largerThan > 0 {
					for _, file := range run.Files {
						fmt.Fprintf(a.Stdout, "  %s (%s)\n", file.Path, pruneBytes(file.Size))
					}
				}
				files += len(run.Files)
			}
			fmt.Fprintf(a.Stdout, "Remove %d files (%s) from %d runs; %d runs protected.\n", files, pruneBytes(plan.RemoveBytes), len(plan.Runs), plan.ProtectedRuns)
			if largerThan == 0 && maxSize > 0 && plan.TotalBytes-plan.RemoveBytes > maxSize {
				fmt.Fprintln(a.Stdout, "Protected outputs prevent reaching the size limit.")
			}
			if dryRun || files == 0 {
				return nil
			}
			fmt.Fprintln(a.Stdout, "Logs and artifacts will be deleted. Task results and failure messages will remain.")
			if !yes {
				fmt.Fprint(a.Stdout, "Proceed? [y/N] ")
				input := a.Stdin
				if input == nil {
					input = os.Stdin
				}
				scanner := bufio.NewScanner(input)
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return err
					}
					fmt.Fprintln(a.Stdout, "Canceled.")
					return nil
				}
				answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if answer != "y" && answer != "yes" {
					fmt.Fprintln(a.Stdout, "Canceled.")
					return nil
				}
			}
			result, err := localci.ApplyPrune(paths, plan)
			fmt.Fprintf(a.Stdout, "Removed %d files (%s); skipped %d runs that changed or became active.\n", result.Files, pruneBytes(result.Bytes), result.SkippedRuns)
			return err
		},
	}
	cmd.Flags().StringVar(&age, "max-age", "30d", "Remove run outputs older than this age (0 disables)")
	cmd.Flags().StringVar(&size, "max-size", "10GiB", "Maximum combined output size; prune oldest runs first (0 disables)")
	cmd.Flags().StringVar(&larger, "larger-than", "0", "Instead remove individual artifact files above this size")
	cmd.Flags().BoolVar(&yes, "yes", false, "Remove outputs without asking for confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview removals without deleting anything")
	return cmd
}

func pruneBytes(bytes int64) string {
	if bytes >= 1<<30 {
		return fmt.Sprintf("%.2f GiB", float64(bytes)/(1<<30))
	}
	if bytes >= 1<<20 {
		return fmt.Sprintf("%.2f MiB", float64(bytes)/(1<<20))
	}
	if bytes >= 1<<10 {
		return fmt.Sprintf("%.2f KiB", float64(bytes)/(1<<10))
	}
	return fmt.Sprintf("%d B", bytes)
}

func (a App) printPruneHint(paths localci.Paths) {
	count, err := (localci.RunRepository{Paths: paths}).OldOutputRunCount(time.Now().Add(-localci.DefaultPruneAge))
	if err == nil && count > 0 {
		fmt.Fprintf(a.Stderr, "%d completed runs are older than 30 days; use `localci prune --dry-run` to check their outputs.\n", count)
	}
}
