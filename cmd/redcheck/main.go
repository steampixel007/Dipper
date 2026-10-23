// redcheck 判定一次 go test 的输出是不是「真红」：把修复改回旧版后，
// 新写的测试必须红在断言上，而不是红在编译、跳过或超时上。
//
// 它不负责改回代码。改回哪一处是语义判断，只有读懂代码的人或 agent 能做；
// 这个程序只回答「这次红算不算数」。
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

// 判定结果。退出码让调用方（人、agent、CI）能机械区分，不必解析输出。
const (
	exitRealRed  = 0 // 真红：有 FAIL，且有断言文本
	exitNotRed   = 1 // 假绿：测试没红，说明这个测试抓不住该 bug
	exitFalseRed = 2 // 假红：编译错、被跳过、超时、panic、数据竞争
	exitUsage    = 3 // 用法错误，或环境不满足
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
		return "真红"
	case verdictNotRed:
		return "假绿"
	default:
		return "假红"
	}
}

// 假红的证据。顺序有意义：编译错要先于 FAIL 判，因为一个包编译不过时
// go test 仍会为其他包打印 FAIL 行，不先判会把编译错当成真红。
var falseRedMarkers = []struct {
	re     *regexp.Regexp
	reason string
}{
	{regexp.MustCompile(`(?m)^# .*\n.*\.go:\d+:\d+: `), "编译失败：测试或被测代码没能编译"},
	{regexp.MustCompile(`(?m)\[build failed\]`), "编译失败：包没能编译"},
	{regexp.MustCompile(`(?m)\[setup failed\]`), "setup 失败：测试的准备工作没跑起来"},
	{regexp.MustCompile(`(?m)^\s*--- SKIP:`), "被跳过：SKIP 不算验证"},
	{regexp.MustCompile(`(?m)^\s*--- FAIL: \w+ \(.*\)\n\s+\w+_test\.go:\d+: panic:`), "panic：不是断言失败"},
	{regexp.MustCompile(`(?m)^panic: `), "panic：不是断言失败"},
	{regexp.MustCompile(`(?m)WARNING: DATA RACE`), "数据竞争：不是断言失败"},
	{regexp.MustCompile(`(?m)^testing: timed out after`), "超时：不是断言失败"},
	{regexp.MustCompile(`(?m)no test files|no tests to run|^testing: warning: no tests to run`), "没有测试被跑到"},
	{regexp.MustCompile(`(?m)undefined: |cannot use |declared and not used|imported and not used`), "编译失败：符号或类型对不上"},
}

// 断言文本的行首形态：测试文件里的 t.Errorf / t.Fatalf 输出，或标准库的
// 断言格式。go test 把测试的报错打印在 `--- FAIL:` 行之前，所以要往前找。
var (
	reFailLine    = regexp.MustCompile(`(?m)^\s*--- FAIL: ([\w/]+)`)
	reAssertion   = regexp.MustCompile(`^\s+\S+_test\.go:\d+: .+`)
	reSummaryFail = regexp.MustCompile(`(?m)^FAIL\s`)
)

func main() {
	var (
		testPattern = flag.String("test", "", "要跑的测试名，作为 -run 的正则；留空跑全部")
		pkg         = flag.String("pkg", "./...", "要跑的包")
		outputFile  = flag.String("out", "", "把 go test 的完整输出存到这个文件；留空则不存")
		expect      = flag.String("expect", "fail", "期望的结果：fail（改回旧代码后，应当真红）或 pass（带着修复，应当通过）")
		count       = flag.Int("count", 1, "go test -count；偶发的红不是证据，可用 -count=5 连跑")
		race        = flag.Bool("race", false, "加 -race")
		timeout     = flag.String("timeout", "10m", "go test -timeout")
		quiet       = flag.Bool("quiet", false, "只打印结论，不打印判定依据")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "redcheck: 判定 go test 的红是不是真红。\n\n用法:\n")
		fmt.Fprintf(os.Stderr, "  go run ./cmd/redcheck -test '^TestFoo$'\n\n")
		fmt.Fprintf(os.Stderr, "先自己把修复改回旧版（或保留修复），再跑这一条：\n")
		fmt.Fprintf(os.Stderr, "  -expect fail  改回旧版后跑，判定这次红算不算数\n")
		fmt.Fprintf(os.Stderr, "  -expect pass  带着修复跑，判定测试是否真的通过\n\n参数:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n退出码: 0 真红/通过, 1 假绿/未通过, 2 假红, 3 用法或环境错误\n")
	}
	flag.Parse()

	if *testPattern == "" && !*quiet {
		fmt.Fprintln(os.Stderr, "redcheck: 没有给 -test；将跑全部测试。")
	}
	if *expect != "fail" && *expect != "pass" {
		fmt.Fprintf(os.Stderr, "redcheck: -expect 只能是 fail 或 pass，收到 %q\n", *expect)
		os.Exit(exitUsage)
	}
	if *count < 1 {
		fmt.Fprintf(os.Stderr, "redcheck: -count 至少为 1，收到 %d\n", *count)
		os.Exit(exitUsage)
	}
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Fprintln(os.Stderr, "redcheck: PATH 里找不到 go。")
		os.Exit(exitUsage)
	}

	out, runErr := runGoTest(*pkg, *testPattern, *count, *race, *timeout)
	if *outputFile != "" {
		if err := os.MkdirAll(filepath.Dir(absOrDot(*outputFile)), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "redcheck: 建输出目录失败: %v\n", err)
		}
		if err := os.WriteFile(*outputFile, []byte(out), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "redcheck: 写输出文件失败: %v\n", err)
		}
	}

	if *expect == "pass" {
		code := reportPass(out, runErr, *testPattern, *quiet)
		os.Exit(code)
	}
	code := reportFail(out, runErr, *testPattern, *quiet)
	os.Exit(code)
}

// runGoTest 跑 go test 并把 stdout 与 stderr 合起来返回。
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
	// 不设 cmd.Env：测试要用到调用方的环境。设 GOFLAGS 会改变调用方的
	// 构建方式，所以只在需要时追加。
	combined, err := cmd.CombinedOutput()
	header := fmt.Sprintf("$ go %s\n\n", strings.Join(args, " "))
	return header + string(combined), err
}

// reportFail 判定「改回旧版后应当红」的那次运行。
func reportFail(out string, runErr error, pattern string, quiet bool) int {
	v, reason, detail := judge(out)

	if !quiet {
		fmt.Println("── 判定依据 ──")
		fmt.Println(reason)
		if detail != "" {
			fmt.Println()
			fmt.Println(detail)
		}
		fmt.Println()
	}

	switch v {
	case verdictRealRed:
		fmt.Printf("✅ 真红：%s 红在断言上。\n", describeTest(pattern))
		fmt.Println("   这次红算数：测试确实抓到了改回去的那个 bug。")
		if runErr == nil {
			fmt.Println("   注意：go test 的退出码是 0，说明有别的包过了；红在哪个包以上面的 FAIL 行为准。")
		}
		return exitRealRed
	case verdictFalseRed:
		fmt.Printf("❌ 假红：%s\n", reason)
		fmt.Println("   编译错、跳过、超时、panic 都不算验证。这个测试没能证明任何事。")
		fmt.Println("   若是编译错，说明改回去的位置动了接口，换个位置再试。")
		return exitFalseRed
	default:
		fmt.Printf("❌ 假绿：%s 在改回旧版后仍然通过。\n", describeTest(pattern))
		fmt.Println("   这个测试抓不住它声称要守的那个 bug。先让它在旧代码上红，再谈修复。")
		return exitNotRed
	}
}

// reportPass 判定「带着修复应当通过」的那次运行。
func reportPass(out string, runErr error, pattern string, quiet bool) int {
	if runErr == nil {
		if !quiet {
			fmt.Printf("✅ 通过：%s 在带修复的代码上通过。\n", describeTest(pattern))
			fmt.Println("   接下来把修复改回旧版，用 -expect fail 再跑一次。")
		}
		return exitRealRed
	}
	v, reason, _ := judge(out)
	if !quiet {
		fmt.Println("── 判定依据 ──")
		fmt.Println(reason)
		fmt.Println()
	}
	switch v {
	case verdictFalseRed:
		fmt.Printf("❌ 没跑起来：%s\n", reason)
		return exitFalseRed
	default:
		fmt.Printf("❌ 未通过：%s 在带修复的代码上仍然失败。\n", describeTest(pattern))
		return exitNotRed
	}
}

// judge 从 go test 的输出里判定结果。假红先于真红判：一个包编译不过时，
// go test 仍会为其他包打印 FAIL 行，不先判就会把编译错当成真红。
func judge(out string) (verdict, string, string) {
	for _, m := range falseRedMarkers {
		if loc := m.re.FindStringIndex(out); loc != nil {
			return verdictFalseRed, m.reason, excerpt(out, loc[0])
		}
	}

	fails := reFailLine.FindAllStringSubmatch(out, -1)
	if len(fails) == 0 {
		if reSummaryFail.MatchString(out) {
			// 有 FAIL 但没有任何 --- FAIL 行：多半是包级失败
			return verdictFalseRed, "有 FAIL 行但没有具体的测试失败：包级失败，不是断言", firstLines(out, 6)
		}
		return verdictNotRed, "没有任何测试失败", ""
	}

	// 要求失败测试下面紧跟断言文本，证明它红在断言上。
	for _, f := range fails {
		name := f[1]
		if detail, ok := assertionFor(out, name); ok {
			return verdictRealRed,
				fmt.Sprintf("%s 失败，并给出了断言文本", name),
				detail
		}
	}
	seen := make(map[string]bool, len(fails))
	names := make([]string, 0, len(fails))
	for _, f := range fails {
		// 子测试失败时父测试也会打印一行 FAIL，名字会重复。
		if seen[f[1]] {
			continue
		}
		seen[f[1]] = true
		names = append(names, f[1])
	}
	return verdictFalseRed,
		fmt.Sprintf("%s 失败，但没给出断言文本", strings.Join(names, ", ")),
		"FAIL 行上面没有 `_test.go:NN: ...` 形式的报错，无法确认红在断言上。"
}

// assertionFor 找出某个测试的断言文本。go test 的两种排版都要认：
// 带 -v 时先打报错再打 `--- FAIL:`，不带 -v 时先打 `--- FAIL:` 再打报错。
// 两个方向都找，碰到别的测试的边界就停，免得把别人的报错算过来。
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

// isTestBoundary 说这一行是不是另一个测试的开始或结束。
func isTestBoundary(ln string) bool {
	return strings.Contains(ln, "--- FAIL:") || strings.Contains(ln, "--- PASS:") ||
		strings.Contains(ln, "--- SKIP:") || strings.HasPrefix(ln, "=== RUN") ||
		strings.HasPrefix(ln, "=== PAUSE") || strings.HasPrefix(ln, "=== CONT")
}

// excerpt 取出某个位置附近的行，供人核对判定依据。
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
		return "全部测试"
	}
	return pattern
}

func absOrDot(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return "." + string(filepath.Separator) + p
}
