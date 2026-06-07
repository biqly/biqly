//go:build ignore

package main

import (
	"fmt"
	"os"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
)

func main() {
	snap := evalpkg.BaselineSnapshotFromCases(evalpkg.NightlySuiteName, evalpkg.NightlyCases())
	out := "testdata/eval/nightly_baseline.json"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := evalpkg.SaveRunSnapshot(out, snap); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d cases to %s\n", len(snap.Cases), out)
}
