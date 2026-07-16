// Sprint 0 entrypoint. Deliberately minimal: connect, run INFO, print the
// raw reply. No flags, no TUI, no polling loop yet — see ROADMAP.md.
// This exists purely to prove the RESP client in internal/redis works
// against a real Redis instance before anything else is built on top of it.
package main

import (
	"fmt"
	"os"

	"github.com/rekon/rekon/internal/redis"
)

func main() {
	client, err := redis.Connect("localhost:6379")
	if err != nil {
		fmt.Fprintf(os.Stderr, "rekon: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	info, err := client.Info()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rekon: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(info)
}
