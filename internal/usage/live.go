package usage

import (
	"fmt"
	"io"
	"time"
)

const terminalClear = "\033[H\033[2J"

func runLive(cfg Config, out, stderr io.Writer, stop <-chan struct{}) error {
	interval := cfg.RefreshInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		select {
		case <-stop:
			return nil
		default:
		}

		events, warnings, err := loadEvents(cfg)
		if err != nil {
			return err
		}
		for _, w := range warnings {
			if cfg.Debug {
				fmt.Fprintln(stderr, "warning:", w)
			}
		}
		rows := aggregate(events, cfg)
		fmt.Fprint(out, terminalClear)
		renderReport(out, cfg, rows)

		timer := time.NewTimer(interval)
		select {
		case <-stop:
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
