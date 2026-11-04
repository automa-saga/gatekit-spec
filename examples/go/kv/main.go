package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }

	fmt.Println("kv rollout: record format v1 -> v2 (activation 5 -> 6)")
	fmt.Println("----------------------------------------------------")
	if err := Rollout(context.Background(), NewStore(), logf); err != nil {
		fmt.Fprintln(os.Stderr, "rollout failed:", err)
		os.Exit(1)
	}
}
