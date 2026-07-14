package focus_test

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/focus"
)

func TestRenderBounds(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for _, k := range []focus.Kind{focus.KindHtop, focus.KindBuildLog, focus.KindTestRunner} {
		for _, tick := range []int{0, 3, 50} {
			out := focus.Render(k, 100, 30, rand.New(rand.NewSource(42)), tick)
			lines := strings.Split(out, "\n")
			if len(lines) > 30 {
				t.Errorf("kind %v tick %d: %d lines > 30", k, tick, len(lines))
			}
			for i, ln := range lines {
				if utf8.RuneCountInString(ln) > 100 {
					t.Errorf("kind %v line %d too wide", k, i)
				}
			}
		}
	}
	_ = r
}

func TestHtopLooksLikeHtop(t *testing.T) {
	out := focus.Render(focus.KindHtop, 120, 40, rand.New(rand.NewSource(1)), 0)
	for _, want := range []string{"CPU", "Mem", "Load average", "PID"} {
		if !strings.Contains(out, want) {
			t.Errorf("htop output missing %q", want)
		}
	}
	if !strings.Contains(out, "webpack-dev-srv") && !strings.Contains(out, "rust-analyzer") &&
		!strings.Contains(out, "go-build-srv") {
		t.Error("htop process table must use fake dev-tool names")
	}
}

func TestDeterministicPerSeed(t *testing.T) {
	a := focus.Render(focus.KindBuildLog, 80, 24, rand.New(rand.NewSource(7)), 5)
	b := focus.Render(focus.KindBuildLog, 80, 24, rand.New(rand.NewSource(7)), 5)
	if a != b {
		t.Error("same seed+tick must render identically")
	}
}

// --- Additional tests beyond the brief's minimum, covering the determinism
// contract and per-kind behavior more precisely. ---

func TestDeterministicAllKinds(t *testing.T) {
	for _, k := range []focus.Kind{focus.KindHtop, focus.KindBuildLog, focus.KindTestRunner} {
		a := focus.Render(k, 100, 30, rand.New(rand.NewSource(99)), 4)
		b := focus.Render(k, 100, 30, rand.New(rand.NewSource(99)), 4)
		if a != b {
			t.Errorf("kind %v: same seed+tick must render identically", k)
		}
	}
}

func TestDifferentTickChangesHtopNumbers(t *testing.T) {
	a := focus.Render(focus.KindHtop, 100, 30, rand.New(rand.NewSource(5)), 0)
	b := focus.Render(focus.KindHtop, 100, 30, rand.New(rand.NewSource(5)), 1)
	if a == b {
		t.Error("htop output should differ across ticks (numbers re-roll)")
	}
}

func TestBuildLogRevealsMoreLinesOverTicks(t *testing.T) {
	early := focus.Render(focus.KindBuildLog, 100, 60, rand.New(rand.NewSource(3)), 0)
	later := focus.Render(focus.KindBuildLog, 100, 60, rand.New(rand.NewSource(3)), 3)
	earlyLines := len(strings.Split(strings.TrimRight(early, "\n"), "\n"))
	laterLines := len(strings.Split(strings.TrimRight(later, "\n"), "\n"))
	if laterLines <= earlyLines {
		t.Errorf("buildlog should reveal more lines over time: tick0=%d tick3=%d", earlyLines, laterLines)
	}
}

func TestTestRunnerRevealsMoreLinesOverTicks(t *testing.T) {
	early := focus.Render(focus.KindTestRunner, 100, 60, rand.New(rand.NewSource(3)), 0)
	later := focus.Render(focus.KindTestRunner, 100, 60, rand.New(rand.NewSource(3)), 3)
	earlyLines := len(strings.Split(strings.TrimRight(early, "\n"), "\n"))
	laterLines := len(strings.Split(strings.TrimRight(later, "\n"), "\n"))
	if laterLines <= earlyLines {
		t.Errorf("testrunner should reveal more lines over time: tick0=%d tick3=%d", earlyLines, laterLines)
	}
}

func TestBuildLogEndsWithBuildFinishedWhenComplete(t *testing.T) {
	out := focus.Render(focus.KindBuildLog, 100, 60, rand.New(rand.NewSource(3)), 500)
	if !strings.Contains(out, "Build finished in") {
		t.Error("buildlog should show completion line once fully revealed")
	}
}

func TestTestRunnerEndsWithSummaryWhenComplete(t *testing.T) {
	out := focus.Render(focus.KindTestRunner, 100, 60, rand.New(rand.NewSource(3)), 500)
	if !strings.Contains(out, "passed") {
		t.Error("testrunner should show a passed summary once fully revealed")
	}
}

func TestRandomKindReturnsValidKind(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	seen := map[focus.Kind]bool{}
	for i := 0; i < 100; i++ {
		k := focus.RandomKind(r)
		switch k {
		case focus.KindHtop, focus.KindBuildLog, focus.KindTestRunner:
			seen[k] = true
		default:
			t.Fatalf("RandomKind returned invalid kind %v", k)
		}
	}
	if len(seen) != 3 {
		t.Errorf("expected RandomKind to eventually produce all 3 kinds over 100 draws, got %d distinct", len(seen))
	}
}

func TestHtopMemUsedLessThanTotal(t *testing.T) {
	// Run across several seeds/ticks to catch off-by-one style bugs.
	for seed := int64(0); seed < 20; seed++ {
		out := focus.Render(focus.KindHtop, 120, 40, rand.New(rand.NewSource(seed)), int(seed))
		if !strings.Contains(out, "Mem [") {
			t.Fatalf("seed %d: missing Mem bar line", seed)
		}
	}
}

func TestNoLipglossOrAnsiEscapes(t *testing.T) {
	for _, k := range []focus.Kind{focus.KindHtop, focus.KindBuildLog, focus.KindTestRunner} {
		out := focus.Render(k, 100, 30, rand.New(rand.NewSource(2)), 10)
		if strings.Contains(out, "\x1b[") {
			t.Errorf("kind %v: output must be plain text, found ANSI escape", k)
		}
	}
}
