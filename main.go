package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"dialtone-watcher/internal/watcher"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return nil
	}

	switch args[0] {
	case "start":
		return startWatcher()
	case "stop":
		return stopWatcher()
	case "summary":
		return printSummary()
	case "__run":
		return watcher.RunDaemon()
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		printHelp()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func startWatcher() error {
	status, err := watcher.LoadStatus()
	if err == nil && status.PID > 0 {
		if process, findErr := os.FindProcess(status.PID); findErr == nil {
			if signalErr := process.Signal(syscall.Signal(0)); signalErr == nil {
				fmt.Printf("dialtone-watcher is already running with pid %d\n", status.PID)
				return nil
			}
		}
	}

	pid, err := watcher.StartDetached()
	if err != nil {
		return err
	}

	fmt.Printf("dialtone-watcher started with pid %d\n", pid)
	return nil
}

func stopWatcher() error {
	status, err := watcher.LoadStatus()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("dialtone-watcher is not running")
			return nil
		}
		return err
	}

	process, err := os.FindProcess(status.PID)
	if err != nil {
		return err
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	fmt.Printf("sent stop signal to pid %d\n", status.PID)
	return nil
}

func printSummary() error {
	summary, err := watcher.LoadSummary()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No watcher state found. Start it with: ./dialtone-watcher start")
			return nil
		}
		return err
	}

	fmt.Printf("Watcher running: %t\n", summary.Running)
	if summary.PID > 0 {
		fmt.Printf("Watcher pid: %d\n", summary.PID)
	}
	fmt.Printf("Polls completed: %d\n", summary.PollCount)
	fmt.Printf("Tracked processes: %d\n", summary.TrackedProcessCount)
	fmt.Printf("Tracked domains: %d\n", summary.TrackedDomainCount)
	fmt.Printf(
		"Hardware: %s on %s (%s, %.1f GB RAM, %d logical cores)\n",
		summary.Hardware.Hostname,
		summary.Hardware.Platform,
		summary.Hardware.CPUModel,
		summary.Hardware.TotalMemoryGB,
		summary.Hardware.CPULogicalCores,
	)

	topProcesses := summary.TopProcesses
	if len(topProcesses) == 0 && summary.TopProcess.PID > 0 {
		topProcesses = []watcher.ProcessSnapshot{summary.TopProcess}
	}

	if len(topProcesses) > 0 {
		fmt.Println("Interesting processes:")
		for i, proc := range topProcesses {
			fmt.Printf(
				"  %d. pid=%d name=%s cpu=%.2f%% rss=%.1f MB seen=%d polls\n",
				i+1,
				proc.PID,
				proc.Name,
				proc.CPUPercent,
				proc.MemoryRSSMB,
				proc.PollsSeen,
			)
		}
	} else {
		fmt.Println("Interesting processes: none recorded yet")
	}

	if len(summary.TopDomains) > 0 {
		fmt.Println("Interesting domains:")
		for i, domain := range summary.TopDomains {
			fmt.Printf(
				"  %d. domain=%s rx=%s tx=%s seen=%d polls\n",
				i+1,
				domain.Domain,
				formatBytes(domain.RXBytes),
				formatBytes(domain.TXBytes),
				domain.PollsSeen,
			)
		}
	} else {
		fmt.Println("Interesting domains: none recorded yet")
	}

	return nil
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}

	divisor, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		divisor *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exp])
}

func printHelp() {
	fmt.Println("dialtone-watcher")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ./dialtone-watcher start")
	fmt.Println("  ./dialtone-watcher stop")
	fmt.Println("  ./dialtone-watcher summary")
	fmt.Println("  ./dialtone-watcher help")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start    start the background process watcher")
	fmt.Println("  stop     stop the background process watcher")
	fmt.Println("  summary  print watcher poll counts and a compact runtime summary")
	fmt.Println("  help     print this menu")
}
