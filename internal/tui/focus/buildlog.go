package focus

import (
	"fmt"
	"math/rand"
	"strings"
)

var buildProjects = []string{
	"dashboard-frontend", "admin-panel", "customer-portal",
	"internal-tools", "analytics-ui", "design-system",
}

var buildModules = []string{
	"src/index.tsx", "src/App.tsx", "src/store/index.ts",
	"src/store/slices/authSlice.ts", "src/components/Header/Header.tsx",
	"src/components/Sidebar/Sidebar.tsx", "src/components/Dashboard/Dashboard.tsx",
	"src/components/Dashboard/Chart.tsx", "src/components/Auth/Login.tsx",
	"src/hooks/useAuth.ts", "src/utils/api.ts", "src/styles/globals.css",
}

var cargoCrates = []string{
	"libc", "proc-macro2", "syn", "quote", "serde", "serde_json",
	"tokio", "futures-util", "rand", "regex", "clap", "anyhow",
}

// renderBuildLog renders a fake webpack- or cargo-style build log. The full
// log body is generated once, deterministically, from r; tick controls how
// many of its lines have been "typed" onto the screen so far (min(tick*2+8,
// total)). Once every line is revealed, a "Build finished in N.NNs"
// completion line is appended — deterministically, since it too is derived
// from r rather than wall-clock time.
func renderBuildLog(w, h int, r *rand.Rand, tick int) []string {
	body := buildLogBody(r)

	reveal := tick*2 + 8
	if reveal < 0 {
		reveal = 8
	}
	if reveal > len(body) {
		reveal = len(body)
	}
	if reveal < 0 {
		reveal = 0
	}

	lines := make([]string, 0, reveal+2)
	lines = append(lines, body[:reveal]...)

	if reveal == len(body) {
		buildTime := 2.0 + r.Float64()*40.0
		lines = append(lines, "", fmt.Sprintf("Build finished in %.2fs", buildTime))
	}

	return lines
}

// buildLogBody deterministically generates the full (un-revealed) content of
// a fake build log from r: a webpack-flavored log about half the time, a
// cargo-flavored one the rest, mirroring the two build tools wrkmon
// pretends to be running.
func buildLogBody(r *rand.Rand) []string {
	if r.Intn(2) == 0 {
		return webpackBuildBody(r)
	}
	return cargoBuildBody(r)
}

func webpackBuildBody(r *rand.Rand) []string {
	project := buildProjects[r.Intn(len(buildProjects))]
	nodeMajor := []int{16, 18, 20}[r.Intn(3)]
	nodeVer := fmt.Sprintf("%d.%d.%d", nodeMajor, r.Intn(20), r.Intn(10))
	webpackVer := fmt.Sprintf("5.%d.%d", 75+r.Intn(18), r.Intn(4))

	lines := []string{
		"$ npm run build",
		"",
		fmt.Sprintf("> %s@2.%d.%d build", project, r.Intn(10), r.Intn(16)),
		"> webpack --mode production --config webpack.prod.js",
		"",
		"[webpack-cli] Compiling...",
		"",
	}

	moduleCount := min(len(buildModules), 4+r.Intn(6))
	for _, idx := range r.Perm(len(buildModules))[:moduleCount] {
		size := 2 + r.Intn(579)
		lines = append(lines, fmt.Sprintf("  %s %d KiB [built]", buildModules[idx], size))
	}
	lines = append(lines, "")

	lines = append(lines,
		"asset                                 size       chunks  name",
		strings.Repeat("-", 67),
	)

	type chunk struct {
		name string
		size int
	}
	chunks := []chunk{
		{"main", 120 + r.Intn(261)},
		{"vendor", 400 + r.Intn(501)},
		{"runtime", 2 + r.Intn(11)},
	}
	total := 0
	for _, c := range chunks {
		total += c.size
		hash := fmt.Sprintf("%08x", r.Uint32())
		pad := 30 - len(c.name)
		if pad < 1 {
			pad = 1
		}
		lines = append(lines, fmt.Sprintf("  %s.%s.js%s%5d KiB  [%s]", c.name, hash, strings.Repeat(" ", pad), c.size, c.name))
	}

	buildMs := 8500 + r.Intn(36500)
	lines = append(lines,
		"",
		fmt.Sprintf("webpack %s compiled successfully in %d ms (node %s)", webpackVer, buildMs, nodeVer),
		"",
		fmt.Sprintf("  %d modules", 180+r.Intn(241)),
		fmt.Sprintf("  %d assets", len(chunks)),
		fmt.Sprintf("  %d KiB total", total),
	)

	return lines
}

func cargoBuildBody(r *rand.Rand) []string {
	target := []string{"debug", "release"}[r.Intn(2)]
	lines := []string{
		fmt.Sprintf("$ cargo build --%s", target),
	}

	crateCount := min(len(cargoCrates), 6+r.Intn(5))
	for _, idx := range r.Perm(len(cargoCrates))[:crateCount] {
		lines = append(lines, fmt.Sprintf("   Compiling %s v%d.%d.%d", cargoCrates[idx], r.Intn(2), r.Intn(20), r.Intn(10)))
	}

	crateVer := fmt.Sprintf("0.%d.%d", 1+r.Intn(9), r.Intn(20))
	lines = append(lines, fmt.Sprintf("   Compiling app-crate v%s", crateVer))

	buildSecs := 2.0 + r.Float64()*30.0
	profile := "unoptimized + debuginfo"
	if target == "release" {
		profile = "optimized"
	}
	lines = append(lines, fmt.Sprintf("    Finished %s [%s] target(s) in %.2fs", target, profile, buildSecs))

	return lines
}
