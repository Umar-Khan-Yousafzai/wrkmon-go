package focus

import (
	"fmt"
	"math/rand"
	"strings"
)

// cpuBarOptions and memTotalOptions mirror the ranges wrkmon v1 used for its
// fake htop screen (ports/utils/stealth.py + ui/screens/focus.py): a plausible
// core count and a plausible total RAM size, picked once per render.
var cpuBarOptions = []int{4, 8, 12, 16}
var memTotalOptions = []int{8, 16, 32, 64}

// fakeUsers are process owners shown in the fake process table. Mostly the
// current user, with a couple of system-y accounts for flavor.
var fakeUsers = []string{"umer", "root", "www-data", "node", "systemd+"}

// niceValues mirrors typical `ps`/`top` nice values, weighted toward 0.
var niceValues = []int{0, 0, 0, 0, -5, -10, 10, 19}

// processStates mirrors typical process states, weighted toward Sleeping.
var processStates = []string{"S", "S", "S", "S", "R", "D"}

// renderHtop renders a fake `htop`-style process monitor: a header (uptime,
// task count, load average), one bar per fake CPU core, Mem/Swp bars, and a
// PID/USER/.../Command table populated with fakeProcessNames so the process
// list reads as ordinary dev tooling (webpack, tsc, cargo-watch, ...).
//
// All numbers are derived from a *rand.Rand re-seeded from (r, tick) — see
// seededRand — so the same tick always reproduces the same screen, and a
// different tick re-rolls everything (the "the numbers changed" animation
// effect Render's tick parameter is for).
func renderHtop(w, h int, r *rand.Rand, tick int) []string {
	rr := seededRand(r, tick)

	cpuCount := cpuBarOptions[rr.Intn(len(cpuBarOptions))]
	memTotal := memTotalOptions[rr.Intn(len(memTotalOptions))]

	// Fixed (non-CPU-bar, non-process-row) line count: title, load-avg,
	// blank, blank (after cpu bars), Mem, Swp, blank, table header,
	// separator = 9 lines.
	const overhead = 9
	const minRows = 10
	const maxRows = 15

	// If the requested cpu count would leave no room for a readable
	// process table, step down to a smaller (still plausible) core count
	// rather than let the table collapse to nothing.
	for cpuCount+overhead+minRows > h && cpuCount > cpuBarOptions[0] {
		for i := len(cpuBarOptions) - 1; i >= 0; i-- {
			if cpuBarOptions[i] < cpuCount {
				cpuCount = cpuBarOptions[i]
				break
			}
		}
	}

	rows := h - overhead - cpuCount
	if rows > maxRows {
		rows = maxRows
	}
	if rows < 1 {
		rows = 1
	}

	tasksTotal := 180 + rr.Intn(161)
	tasksRunning := 1 + rr.Intn(5)
	uptimeH := rr.Intn(73)
	uptimeM := rr.Intn(60)
	load1 := 0.05 + rr.Float64()*float64(cpuCount)*0.6
	load5 := 0.05 + rr.Float64()*float64(cpuCount)*0.5
	load15 := 0.05 + rr.Float64()*float64(cpuCount)*0.4

	lines := []string{
		fmt.Sprintf("htop - up %d:%02d, %d tasks, %d running", uptimeH, uptimeM, tasksTotal, tasksRunning),
		fmt.Sprintf("CPU  Load average: %.2f, %.2f, %.2f", load1, load5, load15),
		"",
	}

	for i := 1; i <= cpuCount; i++ {
		usage := 0.5 + rr.Float64()*94.5 // 0.5 .. 95.0, matches v1's range
		lines = append(lines, cpuBarLine(i, usage))
	}
	lines = append(lines, "")

	memUsed := clampF(float64(memTotal)*0.30+rr.Float64()*float64(memTotal)*0.45, 0, float64(memTotal)*0.98)
	swapTotal := memTotal / 2
	if swapTotal < 1 {
		swapTotal = 1
	}
	swapUsed := clampF(0.1+rr.Float64()*float64(swapTotal)*0.25, 0, float64(swapTotal)*0.95)

	lines = append(lines, memBarLine("Mem", memUsed, float64(memTotal)))
	lines = append(lines, memBarLine("Swp", swapUsed, float64(swapTotal)))
	lines = append(lines, "")

	lines = append(lines, "  PID USER      PRI  NI  VIRT   RES   SHR S  CPU%  MEM%   TIME+  Command")
	lines = append(lines, strings.Repeat("-", min(w, 90)))

	usedPIDs := map[int]bool{}
	for i := 0; i < rows; i++ {
		lines = append(lines, processRow(rr, usedPIDs))
	}

	return lines
}

// processRow renders one fake `htop` process-table row with a unique
// 4-to-5-digit PID and a command drawn from fakeProcessNames.
func processRow(rr *rand.Rand, usedPIDs map[int]bool) string {
	pid := 1000 + rr.Intn(99000) // 1000..99999: always 4 or 5 digits
	for usedPIDs[pid] {
		pid = 1000 + rr.Intn(99000)
	}
	usedPIDs[pid] = true

	user := fakeUsers[rr.Intn(len(fakeUsers))]
	pri := rr.Intn(40)
	ni := niceValues[rr.Intn(len(niceValues))]
	virt := 64 + rr.Intn(4033)
	res := 8 + rr.Intn(505)
	if res > virt {
		res = virt
	}
	shr := rr.Intn(res + 1)
	state := processStates[rr.Intn(len(processStates))]
	cpuPct := rr.Float64() * 18.0
	memPct := rr.Float64() * 8.0
	minutes := rr.Intn(121)
	seconds := rr.Intn(60)
	hundredths := rr.Intn(100)
	cmd := fakeProcessNames[rr.Intn(len(fakeProcessNames))]

	return fmt.Sprintf(
		"  %5d %-9s %3d %3d %5dM %4dM %4dM %s  %5.1f %5.1f %3d:%02d.%02d  %s",
		pid, user, pri, ni, virt, res, shr, state, cpuPct, memPct, minutes, seconds, hundredths, cmd,
	)
}

// cpuBarLine renders one `  N [||||     NN.N%]` core-usage row.
func cpuBarLine(index int, usagePct float64) string {
	const width = 30
	filled := int(clampF(usagePct, 0, 100) / 100 * width)
	bar := strings.Repeat("|", filled) + strings.Repeat(" ", width-filled)
	return fmt.Sprintf("  %2d [%s %5.1f%%]", index, bar, usagePct)
}

// memBarLine renders a `Label [||||     usedG/totalG]` bar, used for both
// the Mem and Swp rows.
func memBarLine(label string, used, total float64) string {
	const width = 30
	frac := 0.0
	if total > 0 {
		frac = used / total
	}
	filled := int(clampF(frac, 0, 1) * width)
	bar := strings.Repeat("|", filled) + strings.Repeat(" ", width-filled)
	return fmt.Sprintf("%s [%s %5.1fG/%dG]", label, bar, used, int(total))
}

// clampF clamps v to [lo, hi].
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
