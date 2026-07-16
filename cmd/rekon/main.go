// Sprint 2 entrypoint. Replaces Sprint 1's manual stdin-reading loop
// with bubbletea's program runner, which owns terminal input/output and
// the Model/Update/View lifecycle. This is the first point where Rekon
// gets real (if unstyled) raw-mode keypress handling for free, resolving
// the "q + Enter" simplification logged in TECHNICAL_DEBT.md for Sprint 1.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rekon/rekon/internal/model"
	"github.com/rekon/rekon/internal/poller"
	"github.com/rekon/rekon/internal/redis"
)

func main() {
	client, err := redis.Connect("localhost:6379")
	if err != nil {
		fmt.Fprintf(os.Stderr, "rekon: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	p := poller.New(client, 1*time.Second)
	p.Start()
	defer p.Stop()

	m := model.New(p.Results)

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "rekon: %v\n", err)
		os.Exit(1)
	}
}
