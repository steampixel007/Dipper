package main

import (
	"strings"
	"testing"
)

// The judge is the whole value of this tool, so every shape of input it can
// meet has a test here. The samples are the shapes go test really prints.

func TestJudgeRealRed(t *testing.T) {
	out := `=== RUN   TestFoo
    foo_test.go:42: got 10, want 73
--- FAIL: TestFoo (0.00s)
FAIL
exit status 1
FAIL	dipper.local/dipper/internal/core	0.012s
`
	v, reason, detail := judge(out)
	if v != verdictRealRed {
		t.Fatalf("judged %v, want a real red: %s", v, reason)
	}
	if detail == "" || !contains(detail, "got 10, want 73") {
		t.Fatalf("no assertion text: %q", detail)
	}
}

func TestJudgeRealRedWithoutVerbose(t *testing.T) {
	// Without -v, go test prints --- FAIL before the error. This is the most
	// common shape.
	out := `--- FAIL: TestRealFail (0.00s)
    probe_test.go:8: got 10, want 73
FAIL
FAIL	dipper.local/dipper/internal/core	0.455s
`
	v, reason, detail := judge(out)
	if v != verdictRealRed {
		t.Fatalf("judged %v, want a real red: %s", v, reason)
	}
	if !contains(detail, "got 10, want 73") {
		t.Fatalf("no assertion text: %q", detail)
	}
}

func TestJudgeRealRedVerbose(t *testing.T) {
	// With -v, the error comes before --- FAIL.
	out := `=== RUN   TestFoo
    foo_test.go:42: got 10, want 73
--- FAIL: TestFoo (0.00s)
FAIL
`
	v, reason, detail := judge(out)
	if v != verdictRealRed {
		t.Fatalf("judged %v, want a real red: %s", v, reason)
	}
	if !contains(detail, "got 10, want 73") {
		t.Fatalf("no assertion text: %q", detail)
	}
}

func TestJudgeNotRed(t *testing.T) {
	out := `ok  	dipper.local/dipper/internal/core	0.012s
`
	v, reason, _ := judge(out)
	if v != verdictNotRed {
		t.Fatalf("judged %v, want not red: %s", v, reason)
	}
}

func TestJudgeCompileErrorIsNotRed(t *testing.T) {
	out := `# dipper.local/dipper/internal/core [dipper.local/dipper/internal/core.test]
internal/core/foo.go:10:2: undefined: bar
FAIL	dipper.local/dipper/internal/core [build failed]
FAIL
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgeCompileErrorWinsOverOtherPackagesFail(t *testing.T) {
	// One package does not compile while another really fails: the compile
	// error must be judged first, or the run is miscalled a real red.
	out := `# dipper.local/dipper/internal/source [dipper.local/dipper/internal/source.test]
internal/source/html.go:5:1: syntax error: unexpected EOF
FAIL	dipper.local/dipper/internal/source [build failed]
    core_test.go:88: boom
--- FAIL: TestOther (0.00s)
FAIL
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgeSkipIsNotRed(t *testing.T) {
	out := `=== RUN   TestFoo
    foo_test.go:12: skipping: needs network
--- SKIP: TestFoo (0.00s)
PASS
ok  	dipper.local/dipper/internal/core	0.010s
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgePanicIsNotRed(t *testing.T) {
	out := `=== RUN   TestFoo
--- FAIL: TestFoo (0.01s)
panic: runtime error: index out of range [recovered]
FAIL	dipper.local/dipper/internal/core	0.020s
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgeDataRaceIsNotRed(t *testing.T) {
	out := `=== RUN   TestFoo
==================
WARNING: DATA RACE
Write at 0x00c000 by goroutine 7:
FAIL	dipper.local/dipper/internal/core	0.030s
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgeTimeoutIsNotRed(t *testing.T) {
	out := `panic: test timed out after 10m0s
testing: timed out after 10m0s
FAIL	dipper.local/dipper/internal/core	600.010s
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgeNoTestsToRunIsNotRed(t *testing.T) {
	out := `testing: warning: no tests to run
PASS
ok  	dipper.local/dipper/internal/core	0.010s
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgeFailWithoutAssertionIsNotRed(t *testing.T) {
	// A --- FAIL with no _test.go:NN line under it cannot be confirmed to have
	// failed on an assertion.
	out := `=== RUN   TestFoo
--- FAIL: TestFoo (0.00s)
FAIL
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("judged %v, want a false red: %s", v, reason)
	}
}

func TestJudgeSubtestAssertion(t *testing.T) {
	out := `=== RUN   TestFoo/case_one
    foo_test.go:55: case one: got 1, want 2
    --- FAIL: TestFoo/case_one (0.00s)
=== RUN   TestFoo/case_two
--- FAIL: TestFoo (0.00s)
FAIL
`
	v, reason, detail := judge(out)
	if v != verdictRealRed {
		t.Fatalf("judged %v, want a real red: %s", v, reason)
	}
	if !contains(detail, "case one: got 1, want 2") {
		t.Fatalf("no assertion text: %q", detail)
	}
}

func TestAssertionDoesNotLeakIntoNextTest(t *testing.T) {
	// Two tests fail, only the second prints an assertion. The first must not
	// borrow the second's error.
	out := `--- FAIL: TestBare (0.00s)
--- FAIL: TestReal (0.00s)
    real_test.go:9: got 1, want 2
FAIL	dipper.local/dipper/internal/core	0.010s
`
	if got, ok := assertionFor(out, "TestBare"); ok {
		t.Fatalf("counted another test's error for TestBare: %q", got)
	}
	got, ok := assertionFor(out, "TestReal")
	if !ok {
		t.Fatal("no assertion text for TestReal")
	}
	if !contains(got, "got 1, want 2") {
		t.Fatalf("took the wrong line: %q", got)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
