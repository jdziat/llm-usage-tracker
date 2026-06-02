package usage

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type progressIndicator struct {
	w       io.Writer
	mu      sync.Mutex
	done    chan struct{}
	message string
	running bool
}

func newProgressIndicator(w io.Writer) *progressIndicator {
	return &progressIndicator{w: w, done: make(chan struct{})}
}

func (p *progressIndicator) Start() {
	if p == nil || p.w == nil {
		return
	}
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()
	go p.loop()
}

func (p *progressIndicator) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.done)
	p.mu.Unlock()
	fmt.Fprint(p.w, "\r"+strings.Repeat(" ", 100)+"\r")
}

func (p *progressIndicator) Set(message string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.message = message
	p.mu.Unlock()
}

func (p *progressIndicator) Done(message string) {
	if p == nil || p.w == nil {
		return
	}
	p.mu.Lock()
	p.message = ""
	p.mu.Unlock()
	fmt.Fprint(p.w, "\r"+strings.Repeat(" ", 100)+"\r")
	fmt.Fprintln(p.w, message)
}

func (p *progressIndicator) loop() {
	frames := []string{"-", "\\", "|", "/"}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.mu.Lock()
			msg := p.message
			p.mu.Unlock()
			if msg != "" {
				fmt.Fprintf(p.w, "\r%s %s", frames[i%len(frames)], truncateProgress(msg, 96))
				i++
			}
		}
	}
}

func truncateProgress(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
