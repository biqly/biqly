// Command coveragecheck enforces minimum per-package statement coverage as a CI
// gate. It parses a Go coverage profile (coverage.out) and fails the build when
// any gated package drops below its floor.
//
// Usage:
//
//	go run ./scripts/coveragecheck -profile coverage.out
//
// Floors are intentionally set a few points below current coverage so the gate
// catches regressions without breaking on small, unrelated churn. Ratchet them
// upward over time as coverage improves.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// floors maps a package import path to its minimum acceptable statement
// coverage percentage. Only packages listed here are gated; everything else is
// reported for visibility but never fails the build.
var floors = map[string]float64{
	"github.com/biqly/biqly/internal/dialect":               85.0,
	"github.com/biqly/biqly/internal/datasource/postgres":   85.0,
	"github.com/biqly/biqly/internal/datasource/mysql":      85.0,
	"github.com/biqly/biqly/internal/datasource/clickhouse": 85.0,
	"github.com/biqly/biqly/internal/datasource/sqlserver":  85.0,
	"github.com/biqly/biqly/internal/config":                80.0,
	"github.com/biqly/biqly/internal/dashboard":             80.0,
}

type counts struct {
	covered int
	total   int
}

func main() {
	profile := flag.String("profile", "coverage.out", "path to the Go coverage profile")
	flag.Parse()

	pkgCounts, err := parseProfile(*profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coveragecheck: %v\n", err)
		os.Exit(2)
	}

	var failed []string
	gated := make([]string, 0, len(floors))
	for pkg := range floors {
		gated = append(gated, pkg)
	}
	sort.Strings(gated)

	for _, pkg := range gated {
		floor := floors[pkg]
		c, ok := pkgCounts[pkg]
		if !ok || c.total == 0 {
			printf("FAIL  %-55s no coverage data (floor %.1f%%)\n", pkg, floor)
			failed = append(failed, pkg)
			continue
		}
		pct := 100 * float64(c.covered) / float64(c.total)
		status := "ok  "
		if pct < floor {
			status = "FAIL"
			failed = append(failed, pkg)
		}
		printf("%s  %-55s %5.1f%% (floor %.1f%%)\n", status, pkg, pct, floor)
	}

	if len(failed) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "\ncoveragecheck: %d package(s) below coverage floor: %s\n", len(failed), strings.Join(failed, ", "))
		os.Exit(1)
	}
	printf("\ncoveragecheck: all %d gated package(s) meet their coverage floor\n", len(gated))
}

// printf writes a formatted report line to stdout, discarding the write error
// (stdout failures are not actionable for a CLI reporter).
func printf(format string, a ...any) {
	_, _ = fmt.Fprintf(os.Stdout, format, a...)
}

// parseProfile reads a Go coverage profile and aggregates covered/total
// statements per package import path.
func parseProfile(path string) (map[string]counts, error) {
	// #nosec G304 -- profile is a trusted CI/dev CLI flag, not user-facing input.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open profile: %w", err)
	}
	defer func() { _ = f.Close() }()

	result := make(map[string]counts)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}
		pkg, numStmts, count, ok := parseLine(line)
		if !ok {
			continue
		}
		c := result[pkg]
		c.total += numStmts
		if count > 0 {
			c.covered += numStmts
		}
		result[pkg] = c
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}
	return result, nil
}

// parseLine parses one coverage profile block line of the form:
//
//	import/path/file.go:start.col,end.col numStmts count
//
// returning the package import path (dir of the file), statement count, and the
// execution count.
func parseLine(line string) (pkg string, numStmts, count int, ok bool) {
	colon := strings.LastIndex(line, ":")
	if colon < 0 {
		return "", 0, 0, false
	}
	filePath := line[:colon]
	slash := strings.LastIndex(filePath, "/")
	if slash < 0 {
		return "", 0, 0, false
	}
	pkg = filePath[:slash]

	fields := strings.Fields(line[colon+1:])
	if len(fields) != 3 {
		return "", 0, 0, false
	}
	numStmts, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, 0, false
	}
	count, err = strconv.Atoi(fields[2])
	if err != nil {
		return "", 0, 0, false
	}
	return pkg, numStmts, count, true
}
