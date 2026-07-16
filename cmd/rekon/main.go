// Sprint 1 entrypoint. Proves the polling goroutine and a separate
// input-handling goroutine run independently: Redis is polled on a
// timer and printed as snapshots arrive, while a second goroutine
// waits for the user to type "q" to quit — neither blocks the other.
//
// Still no TUI (that's Sprint 2, via bubbletea). This intentionally
// stays plain stdout printing so the concurrency mechanism itself is
// what's being tested, not rendering.
//
// Known simplification (logged in TECHNICAL_DEBT.md): quitting requires
// "q" followed by Enter, not a single raw keypress. True raw-mode input
// needs either a third-party terminal library or manual termios/syscall
// handling — both are unnecessary complexity for a proof sprint that
// bubbletea will handle properly in Sprint 2 anyway.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

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

	// quit is closed the moment the input goroutine reads "q". main
	// selects on both this and incoming poll results, so neither the
	// polling loop nor input handling can block the other.
	quit := make(chan struct{})
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.TrimSpace(line) == "q" {
				close(quit)
				return
			}
		}
	}()

	fmt.Println("rekon sprint 1 proof — polling every 1s. Type q + Enter to quit.")

	for {
		select {
		case <-quit:
			p.Stop()
			fmt.Println("quitting.")
			return
		case snap, ok := <-p.Results:
			if !ok {
				return
			}
			if snap.Err != nil {
				fmt.Fprintf(os.Stderr, "[%s] poll error: %v\n", snap.Timestamp.Format(time.RFC3339), snap.Err)
				continue
			}
			// Print just the first line of INFO (redis_version) as a
			// lightweight proof that fresh data arrives each second,
			// without flooding the terminal with the full INFO blob
			// on every tick.
			firstLine := strings.SplitN(snap.Info, "\r\n", 2)[0]
			fmt.Printf("[%s] %s\n", snap.Timestamp.Format("15:04:05"), firstLine)
		}
	}
}
