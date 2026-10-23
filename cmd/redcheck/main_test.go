package main

import (
	"strings"
	"testing"
)

// 判定逻辑是这个工具的全部价值，所以它自己的每一种输入形态都要有测试。
// 这里的样例取自 go test 真实会打印的形状。

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
		t.Fatalf("判定为 %v，期望真红：%s", v, reason)
	}
	if detail == "" || !contains(detail, "got 10, want 73") {
		t.Fatalf("断言文本没取到：%q", detail)
	}
}

func TestJudgeRealRedWithoutVerbose(t *testing.T) {
	// 不带 -v 时 go test 先打 --- FAIL 再打报错。这是最常见的形态。
	out := `--- FAIL: TestRealFail (0.00s)
    probe_test.go:8: got 10, want 73
FAIL
FAIL	dipper.local/dipper/internal/core	0.455s
`
	v, reason, detail := judge(out)
	if v != verdictRealRed {
		t.Fatalf("判定为 %v，期望真红：%s", v, reason)
	}
	if !contains(detail, "got 10, want 73") {
		t.Fatalf("断言文本没取到：%q", detail)
	}
}

func TestJudgeRealRedVerbose(t *testing.T) {
	// 带 -v 时先打报错再打 --- FAIL。
	out := `=== RUN   TestFoo
    foo_test.go:42: got 10, want 73
--- FAIL: TestFoo (0.00s)
FAIL
`
	v, reason, detail := judge(out)
	if v != verdictRealRed {
		t.Fatalf("判定为 %v，期望真红：%s", v, reason)
	}
	if !contains(detail, "got 10, want 73") {
		t.Fatalf("断言文本没取到：%q", detail)
	}
}

func TestJudgeNotRed(t *testing.T) {
	out := `ok  	dipper.local/dipper/internal/core	0.012s
`
	v, reason, _ := judge(out)
	if v != verdictNotRed {
		t.Fatalf("判定为 %v，期望假绿：%s", v, reason)
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
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
	}
}

func TestJudgeCompileErrorWinsOverOtherPackagesFail(t *testing.T) {
	// 一个包编译不过，另一个包真红：编译错必须先判，否则会误当真红。
	out := `# dipper.local/dipper/internal/source [dipper.local/dipper/internal/source.test]
internal/source/html.go:5:1: syntax error: unexpected EOF
FAIL	dipper.local/dipper/internal/source [build failed]
    core_test.go:88: boom
--- FAIL: TestOther (0.00s)
FAIL
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
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
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
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
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
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
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
	}
}

func TestJudgeTimeoutIsNotRed(t *testing.T) {
	out := `panic: test timed out after 10m0s
testing: timed out after 10m0s
FAIL	dipper.local/dipper/internal/core	600.010s
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
	}
}

func TestJudgeNoTestsToRunIsNotRed(t *testing.T) {
	out := `testing: warning: no tests to run
PASS
ok  	dipper.local/dipper/internal/core	0.010s
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
	}
}

func TestJudgeFailWithoutAssertionIsNotRed(t *testing.T) {
	// 有 --- FAIL 但下面没有任何 _test.go:NN 的报错：无法确认红在断言上。
	out := `=== RUN   TestFoo
--- FAIL: TestFoo (0.00s)
FAIL
`
	v, reason, _ := judge(out)
	if v != verdictFalseRed {
		t.Fatalf("判定为 %v，期望假红：%s", v, reason)
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
		t.Fatalf("判定为 %v，期望真红：%s", v, reason)
	}
	if !contains(detail, "case one: got 1, want 2") {
		t.Fatalf("断言文本没取到：%q", detail)
	}
}

func TestAssertionDoesNotLeakIntoNextTest(t *testing.T) {
	// 两个测试都失败，只有后一个有断言文本。前一个不能借用后一个的报错。
	out := `--- FAIL: TestBare (0.00s)
--- FAIL: TestReal (0.00s)
    real_test.go:9: got 1, want 2
FAIL	dipper.local/dipper/internal/core	0.010s
`
	if got, ok := assertionFor(out, "TestBare"); ok {
		t.Fatalf("把别的测试的报错算给了 TestBare：%q", got)
	}
	if got, ok := assertionFor(out, "TestReal"); !ok {
		t.Fatal("TestReal 的断言文本没取到")
	} else if !contains(got, "got 1, want 2") {
		t.Fatalf("取到了别的行：%q", got)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
