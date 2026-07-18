// Rekon's entrypoint. Parses connection flags, connects, starts the
// poller, and hands off to bubbletea's program runner, which owns
// terminal input/output and the Model/Update/View lifecycle.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rekon/rekon/internal/model"
	"github.com/rekon/rekon/internal/poller"
	"github.com/rekon/rekon/internal/redis"
)

func main() {
	url := flag.String("url", "localhost:6379", "Redis address to connect to (host:port)")
	interval := flag.Duration("interval", 1*time.Second, "poll interval, e.g. 1s, 500ms")
	flag.Parse()

	client, err := redis.Connect(*url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rekon: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	p := poller.New(client, *interval)
	p.Start()
	defer p.Stop()

	// LocalAddr is captured once, right after connecting — it stays
	// constant for the life of this one persistent connection, so
	// there's no need to re-fetch it on every poll. Passed into Model
	// so the Slowlog panel can filter out Rekon's own polling commands
	// (see metrics.FilterOutSelf and TECHNICAL_DEBT.md's Sprint 4 entry).
	m := model.New(p.Results, client.LocalAddr())

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "rekon: %v\n", err)
		os.Exit(1)
	}
}
