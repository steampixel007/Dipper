// Command redcheck judges whether a `go test` run is a real red: with the fix
// put back to its old version, a new test must fail on an assertion, not on a
// compile error, a skip or a timeout.
//
// It does not put the code back. Which place to revert is a semantic judgment
// that only someone reading the code can make; this program only answers
// whether the red counts.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Exit codes let a caller — a person, an agent, a CI job — tell the outcomes
// apart without parsing the output.
const (
	exitRealRed  = 0 // a real red: a FAIL with assertion text, or the test passed
	exitNotRed   = 1 // not red: the test still passes, so it does not catch the bug
	exitFalseRed = 2 // a false red: compile error, skip, timeout, panic, data race
	exitUsage    = 3 // bad usage, or the environment is not usable
)

type verdict int

const (
	verdictRealRed verdict = iota
	verdictNotRed
	verdictFalseRed
)

func (v verdict) String() string {
	switch v {
	case verdictRealRed:
		return "real red"
	case verdictNotRed:
		return "not red"
	default:
		return "false red"
	}
}

// Evidence of a false red. Order matters: a compile error is judged before a
// FAIL, because when one package does not compile `go test` still prints FAIL
// lines for the others, and judging FAIL first would call it a real red.
var falseRedMarkers = []struct {
	re     *regexp.Regexp
	reason string
}{
	{regexp.MustCompile(`(?m)^# .*\n.*\.go:\d+:\d+: `), "compile error: the test or the code under test did not compile"},
	{regexp.MustCompile(`(?m)\[build failed\]`), "compile error: a package did not build"},
	{regexp.MustCompile(`(?m)\[setup failed\]`), "setup failed: the test's preparation did not run"},
	{regexp.MustCompile(`(?m)^\s*--- SKIP:`), "skipped: a SKIP proves nothing"},
	{regexp.MustCompile(`(?m)^\s*--- FAIL: \w+ \(.*\)\n\s+\w+_test\.go:\d+: panic:`), "panic: not an assertion failure"},
	{regexp.MustCompile(`(?m)^panic: `), "panic: not an assertion failure"},
	{regexp.MustCompile(`(?m)WARNING: DATA RACE`), "data race: not an assertion failure"},
	{regexp.MustCompile(`(?m)^testing: timed out after`), "timeout: not an assertion failure"},
	{regexp.MustCompile(`(?m)no test files|no tests to run|^testing: warning: no tests to run`), "no test was run"},
	{regexp.MustCompile(`(?m)undefined: |cannot use |declared and not used|imported and not used`), "compile error: a symbol or type does not line up"},
}

// The shape of an assertion line: what t.Errorf / t.Fatalf prints from a test
// file. go test prints it before the `--- FAIL:` line under -v and after it
// without -v, so both directions are searched.
var (
	reFailLine    = regexp.MustCompile(`(?m)^\s*--- FAIL: ([\w/]+)`)
	reAssertion   = regexp.MustCompile(`^\s+\S+_test\.go:\d+: .+`)
	reSummaryFail = regexp.MustCompile(`(?m)^FAIL\s`)
)

func main() {
	var (
		testPattern = flag.String("test", "", "test name to run, as a -run regexp; empty runs everything")
		pkg         = flag.String("pkg", "./...", "packages to run")
		outputFile  = flag.String("out", "", "write the full go test output to this file; empty writes nothing")
		expect      = flag.String("expect", "fail", "expected result: fail (with the old code back, it should be a real red) or pass (with the fix, it should pass)")
		count       = flag.Int("count", 1, "go test -count; an intermittent red is not evidence, so -count=5 runs it five times")
		race        = flag.Bool("race", false, "add -race")
		timeout     = flag.String("timeout", "10m", "go test -timeout")
		quiet       = flag.Bool("quiet", false, "print the conclusion only, not the reasoning")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "redcheck: judge whether a go test red is a real red.\n\nUsage:\n")
		fmt.Fprintf(os.Stderr, "  go run ./cmd/redcheck -test '^TestFoo$'\n\n")
		fmt.Fprintf(os.Stderr, "Put the fix back by hand first (or leave it in place), then run one of:\n")
		fmt.Fprintf(os.Stderr, "  -expect fail  with the old code back, judge whether this red counts\n")
		fmt.Fprintf(os.Stderr, "  -expect pass  with the fix in place, judge whether the test passes\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExit codes: 0 real red or passed, 1 not red or did not pass, 2 false red, 3 usage or environment error\n")
	}
	flag.Parse()

	if *testPattern == "" && !*quiet {
		fmt.Fprintln(os.Stderr, "redcheck: no -test given; running every test.")
	}
	if *expect != "fail" && *expect != "pass" {
		fmt.Fprintf(os.Stderr, "redcheck: -expect takes fail or pass, got %q\n", *expect)
		os.Exit(exitUsage)
	}
	if *count < 1 {
		fmt.Fprintf(os.Stderr, "redcheck: -count is at least 1, got %d\n", *count)
		os.Exit(exitUsage)
	}
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Fprintln(os.Stderr, "redcheck: no go in PATH.")
		os.Exit(exitUsage)
	}

	out, runErr := runGoTest(*pkg, *testPattern, *count, *race, *timeout)
	if *outputFile != "" {
		if err := os.MkdirAll(filepath.Dir(absOrDot(*outputFile)), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "redcheck: cannot create the output folder: %v\n", err)
		}
		if err := os.WriteFile(*outputFile, []byte(out), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "redcheck: cannot write the output file: %v\n", err)
		}
	}

	if *expect == "pass" {
		os.Exit(reportPass(out, runErr, *testPattern, *quiet))
	}
	os.Exit(reportFail(out, runErr, *testPattern, *quiet))
}

// runGoTest runs go test and returns its stdout and stderr together.
func runGoTest(pkg, pattern string, count int, race bool, timeout string) (string, error) {
	args := []string{"test", fmt.Sprintf("-count=%d", count), "-timeout", timeout}
	if pattern != "" {
		args = append(args, "-run", pattern)
	}
	if race {
		args = append(args, "-race")
	}
	args = append(args, pkg)

	cmd := exec.Command("go", args...)
	// The environment is not replaced: the tests need the caller's. Adding
	// GOFLAGS would change how the caller's build works.
	combined, err := cmd.CombinedOutput()
	header := fmt.Sprintf("$ go %s\n\n", strings.Join(args, " "))
	return header + string(combined), err
}

// reportFail judges the run where the old code was put back and the test should
// fail.
func reportFail(out string, runErr error, pattern string, quiet bool) int {
	v, reason, detail := judge(out)

	if !quiet {
		fmt.Println("-- how this was judged --")
		fmt.Println(reason)
		if detail != "" {
			fmt.Println()
			fmt.Println(detail)
		}
		fmt.Println()
	}

	switch v {
	case verdictRealRed:
		fmt.Printf("REAL RED: %s failed on an assertion.\n", describeTest(pattern))
		fmt.Println("  This red counts: the test caught the bug that was put back.")
		if runErr == nil {
			fmt.Println("  Note: go test exited 0, so other packages passed; the FAIL lines above say which one failed.")
		}
		return exitRealRed
	case verdictFalseRed:
		fmt.Printf("FALSE RED: %s\n", reason)
		fmt.Println("  A compile error, a skip, a timeout or a panic is not verification. This test proved nothing.")
		fmt.Println("  If it was a compile error, the place put back changed an interface; put back a different place and try again.")
		return exitFalseRed
	default:
		fmt.Printf("NOT RED: %s still passes with the old code back.\n", describeTest(pattern))
		fmt.Println("  This test does not catch the bug it claims to guard. Make it fail on the old code first.")
		return exitNotRed
	}
}

// reportPass judges the run with the fix in place, which should pass.
func reportPass(out string, runErr error, pattern string, quiet bool) int {
	if runErr == nil {
		if !quiet {
			fmt.Printf("PASSED: %s passes with the fix in place.\n", describeTest(pattern))
			fmt.Println("  Next, put the fix back by hand and run again with -expect fail.")
		}
		return exitRealRed
	}
	v, reason, _ := judge(out)
	if !quiet {
		fmt.Println("-- how this was judged --")
		fmt.Println(reason)
		fmt.Println()
	}
	switch v {
	case verdictFalseRed:
		fmt.Printf("DID NOT RUN: %s\n", reason)
		return exitFalseRed
	default:
		fmt.Printf("FAILED: %s still fails with the fix in place.\n", describeTest(pattern))
		return exitNotRed
	}
}

// judge decides the outcome from go test's output. A false red is judged before
// a real red: when one package does not compile, go test still prints FAIL lines
// for the others, and judging FAIL first would call the compile error a red.
func judge(out string) (verdict, string, string) {
	for _, m := range falseRedMarkers {
		if loc := m.re.FindStringIndex(out); loc != nil {
			return verdictFalseRed, m.reason, excerpt(out, loc[0])
		}
	}

	fails := reFailLine.FindAllStringSubmatch(out, -1)
	if len(fails) == 0 {
		if reSummaryFail.MatchString(out) {
			// A FAIL line with no --- FAIL line is a package-level failure.
			return verdictFalseRed, "a FAIL line with no test failure: a package failed, not an assertion", firstLines(out, 6)
		}
		return verdictNotRed, "no test failed", ""
	}

	// The failing test needs assertion text under it to show it failed on an
	// assertion rather than on something else.
	for _, f := range fails {
		name := f[1]
		if detail, ok := assertionFor(out, name); ok {
			return verdictRealRed,
				fmt.Sprintf("%s failed and printed an assertion", name),
				detail
		}
	}
	seen := make(map[string]bool, len(fails))
	names := make([]string, 0, len(fails))
	for _, f := range fails {
		// A failing subtest makes its parent print a FAIL line too, so a name
		// can repeat.
		if seen[f[1]] {
			continue
		}
		seen[f[1]] = true
		names = append(names, f[1])
	}
	return verdictFalseRed,
		fmt.Sprintf("%s failed with no assertion text", strings.Join(names, ", ")),
		"No `_test.go:NN: ...` line sits beside the FAIL line, so there is no telling whether it failed on an assertion."
}

// assertionFor finds a test's assertion text. Both layouts are accepted: under
// -v the error comes before `--- FAIL:`, without -v it comes after. Both
// directions are searched, stopping at another test's boundary so one test's
// error is never counted for another.
func assertionFor(out, testName string) (string, bool) {
	lines := strings.Split(out, "\n")
	at := -1
	for i, ln := range lines {
		if strings.Contains(ln, "--- FAIL: "+testName+" ") || strings.HasSuffix(ln, "--- FAIL: "+testName) {
			at = i
			break
		}
	}
	if at < 0 {
		return "", false
	}

	var before []string
	for i := at - 1; i >= 0; i-- {
		ln := lines[i]
		if isTestBoundary(ln) {
			break
		}
		if reAssertion.MatchString(ln) {
			before = append([]string{strings.TrimRight(ln, " \t")}, before...)
		}
	}
	var after []string
	for i := at + 1; i < len(lines); i++ {
		ln := lines[i]
		if isTestBoundary(ln) || strings.HasPrefix(ln, "FAIL") || strings.HasPrefix(ln, "ok  ") {
			break
		}
		if reAssertion.MatchString(ln) {
			after = append(after, strings.TrimRight(ln, " \t"))
		}
	}

	got := append(before, after...)
	if len(got) == 0 {
		return "", false
	}
	return strings.Join(got, "\n"), true
}

// isTestBoundary says whether a line starts or ends another test.
func isTestBoundary(ln string) bool {
	return strings.Contains(ln, "--- FAIL:") || strings.Contains(ln, "--- PASS:") ||
		strings.Contains(ln, "--- SKIP:") || strings.HasPrefix(ln, "=== RUN") ||
		strings.HasPrefix(ln, "=== PAUSE") || strings.HasPrefix(ln, "=== CONT")
}

// excerpt takes the lines around a position, so a reader can check the verdict.
func excerpt(out string, at int) string {
	lines := strings.Split(out[:at], "\n")
	from := len(lines) - 4
	if from < 0 {
		from = 0
	}
	tail := strings.Split(out[at:], "\n")
	to := 4
	if to > len(tail) {
		to = len(tail)
	}
	got := append(lines[from:], tail[:to]...)
	return strings.Join(got, "\n")
}

func firstLines(out string, n int) string {
	lines := strings.Split(out, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func describeTest(pattern string) string {
	if pattern == "" {
		return "every test"
	}
	return pattern
}

func absOrDot(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return "." + string(filepath.Separator) + p
}
