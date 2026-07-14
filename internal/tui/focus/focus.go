// Package focus renders fake "work" screens (htop, a build log, a test
// runner) used by wrkmon's focus mode: when the player is put into focus
// mode, the TUI shows one of these full-screen text renders instead of the
// player UI, so at a glance the terminal looks like ordinary dev tooling.
//
// This package is intentionally standalone: it has no dependency on Bubble
// Tea, lipgloss, or the rest of wrkmon. Render returns plain text only — no
// ANSI escapes, no styling. The caller (the TUI layer) is responsible for
// drawing that text inside its own frame/style.
package focus

import "math/rand"

// Kind identifies which fake screen to render.
type Kind int

const (
	// KindHtop renders a fake `htop`-style process monitor.
	KindHtop Kind = iota
	// KindBuildLog renders a fake webpack/cargo-style build log.
	KindBuildLog
	// KindTestRunner renders a fake pytest-style test run.
	KindTestRunner
)

// kindCount is the number of valid Kind values, used by RandomKind.
const kindCount = 3

// String returns a human-readable name for the kind, mostly useful for
// logging/debugging and test failure messages.
func (k Kind) String() string {
	switch k {
	case KindHtop:
		return "htop"
	case KindBuildLog:
		return "buildlog"
	case KindTestRunner:
		return "testrunner"
	default:
		return "unknown"
	}
}

// RandomKind picks one of the three Kind values uniformly at random using r.
func RandomKind(r *rand.Rand) Kind {
	return Kind(r.Intn(kindCount))
}

// Render produces a full-screen fake work-screen for the given kind, sized
// to fit within w columns by h rows. The result always has at most h lines
// (separated by "\n", no trailing newline), and every line has at most w
// runes.
//
// r drives all randomness. tick advances the "animation": for KindHtop the
// numbers re-roll every tick (deterministically, seeded from r's initial
// state mixed with tick) so the screen looks alive; for KindBuildLog and
// KindTestRunner, tick controls how many lines of the (deterministic, seeded
// from r) log have been "typed" so far — higher tick reveals more lines,
// until the whole log is shown and a completion/summary line is appended.
//
// Render is a pure function of (kind, w, h, the state of r at call time,
// tick): calling it twice with freshly-seeded *rand.Rand values holding the
// same seed and the same tick yields byte-identical output.
func Render(kind Kind, w, h int, r *rand.Rand, tick int) string {
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}

	var lines []string
	switch kind {
	case KindHtop:
		lines = renderHtop(w, h, r, tick)
	case KindBuildLog:
		lines = renderBuildLog(w, h, r, tick)
	case KindTestRunner:
		lines = renderTestRunner(w, h, r, tick)
	default:
		lines = renderHtop(w, h, r, tick)
	}

	return clip(lines, w, h)
}

// clip enforces the output contract: at most h lines, each at most w runes
// wide. It never pads — shorter output is left as-is.
func clip(lines []string, w, h int) string {
	if len(lines) > h {
		lines = lines[:h]
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		out[i] = clipRunes(ln, w)
	}
	return joinLines(out)
}

// clipRunes truncates s to at most w runes, respecting UTF-8 boundaries.
func clipRunes(s string, w int) string {
	if w < 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == w {
			return s[:i]
		}
		count++
	}
	return s
}

// joinLines joins lines with "\n". Equivalent to strings.Join(lines, "\n").
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	total := len(lines) - 1
	for _, l := range lines {
		total += len(l)
	}
	buf := make([]byte, 0, total)
	for i, l := range lines {
		if i > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, l...)
	}
	return string(buf)
}

// seededRand derives a fresh, deterministic *rand.Rand from r's current
// state mixed with an extra integer (typically tick). This is how
// KindHtop gets different-but-deterministic numbers per tick. Reading
// r.Int63() advances the caller's r — that's fine here: callers pass a
// freshly-seeded r per Render, and each derived stream depends only on
// (the drawn base, mix), so output stays deterministic for a given
// (seed, call order).
func seededRand(r *rand.Rand, mix int) *rand.Rand {
	base := uint64(r.Int63())
	blend := uint64(mix)*2654435761 + 0x9E3779B97F4A7C15
	seed := int64(base ^ blend)
	return rand.New(rand.NewSource(seed))
}

// fakeProcessNames are process names that look like legitimate dev tooling,
// ported from wrkmon v1's stealth.py FAKE_PROCESS_NAMES.
var fakeProcessNames = []string{
	"node-inspector",
	"webpack-dev-srv",
	"vite-hmr-watch",
	"eslint-daemon",
	"tsc-watch",
	"pytest-runner",
	"cargo-watch",
	"go-build-srv",
	"rust-analyzer",
	"prettier-fmt",
}
