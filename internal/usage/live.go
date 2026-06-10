package usage

import (
	"fmt"
	"io"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

const terminalClear = "\033[H\033[2J"

func runLive(q core.Query, opts RenderOptions, out, stderr io.Writer, stop <-chan struct{}) error {
	interval := opts.refreshInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		select {
		case <-stop:
			return nil
		default:
		}

		res, err := core.LoadAndAggregate(q, nil)
		if err != nil {
			return err
		}
		for _, w := range res.Warnings {
			if opts.Debug {
				fmt.Fprintln(stderr, "warning:", warningString(w))
			}
		}
		fmt.Fprint(out, terminalClear)
		renderReport(out, res, q, opts)

		timer := time.NewTimer(interval)
		select {
		case <-stop:
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
