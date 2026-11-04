package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("pingpong: rolling a message format v1 → v2 across two peers")
	fmt.Println("----------------------------------------------------------")
	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	if err := Run(logf); err != nil {
		fmt.Fprintln(os.Stderr, "rollout failed:", err)
		os.Exit(1)
	}
}
