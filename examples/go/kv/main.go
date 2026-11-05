package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }

	fmt.Println("kv: record format v1 -> v2 (activation 5 -> 6)")
	fmt.Println("----------------------------------------------")

	store := NewStore()
	kv, err := NewKV(store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init failed:", err)
		os.Exit(1)
	}
	if err := Rollout(context.Background(), store, kv, logf); err != nil {
		fmt.Fprintln(os.Stderr, "rollout failed:", err)
		os.Exit(1)
	}
}
