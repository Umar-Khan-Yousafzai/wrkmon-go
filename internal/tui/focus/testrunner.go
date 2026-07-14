package focus

import (
	"fmt"
	"math/rand"
	"strings"
)

var testProjects = []string{
	"backend-api", "data-pipeline", "ml-service",
	"auth-service", "notification-service", "core-lib",
}

var testFiles = []string{
	"tests/unit/test_auth.py", "tests/unit/test_users.py",
	"tests/unit/test_models.py", "tests/unit/test_validators.py",
	"tests/unit/test_utils.py", "tests/unit/test_cache.py",
	"tests/integration/test_api_endpoints.py",
	"tests/integration/test_database.py",
}

var testNames = []string{
	"test_create_user", "test_login_valid_credentials",
	"test_login_invalid_password", "test_token_refresh",
	"test_user_profile_update", "test_list_pagination",
	"test_search_filter", "test_permission_denied",
	"test_rate_limiting", "test_input_validation",
	"test_cache_invalidation", "test_database_transaction",
}

// renderTestRunner renders a fake pytest-style test run. The full list of
// test-result lines is generated once, deterministically, from r; tick
// controls how many of those lines have run so far (min(tick*2+8, total)).
// Once every test has "run", a green summary line ("=== N passed in
// N.NNs ===") is appended.
func renderTestRunner(w, h int, r *rand.Rand, tick int) []string {
	body, passed := testRunnerBody(r)

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
		duration := 2.0 + r.Float64()*26.0
		bar := strings.Repeat("=", 20)
		lines = append(lines, "", fmt.Sprintf("%s %d passed in %.2fs %s", bar, passed, duration, bar))
	}

	return lines
}

// testRunnerBody deterministically generates the full (un-revealed) list of
// pytest-style result lines from r, plus how many of them passed.
func testRunnerBody(r *rand.Rand) (lines []string, passed int) {
	project := testProjects[r.Intn(len(testProjects))]
	pyVer := fmt.Sprintf("3.%d.%d", []int{10, 11, 12}[r.Intn(3)], r.Intn(9))
	pytestVer := fmt.Sprintf("7.%d.%d", 2+r.Intn(3), r.Intn(4))
	modName := strings.ReplaceAll(project, "-", "_")

	numCases := 12 + r.Intn(9) // 12..20 tests, matches v1's ballpark

	lines = []string{
		fmt.Sprintf("$ python -m pytest tests/ -v --tb=short --cov=%s", modName),
		"",
		strings.Repeat("=", 20) + " test session starts " + strings.Repeat("=", 19),
		fmt.Sprintf("platform linux -- Python %s, pytest-%s", pyVer, pytestVer),
		fmt.Sprintf("rootdir: /home/dev/projects/%s", project),
		"configfile: pyproject.toml",
		fmt.Sprintf("collected %d items", numCases),
		"",
	}

	for i := 0; i < numCases; i++ {
		file := testFiles[r.Intn(len(testFiles))]
		name := testNames[r.Intn(len(testNames))]
		pct := int(float64(i+1) / float64(numCases) * 100)

		outcome := "PASSED"
		if r.Intn(10) == 0 { // ~10% skipped, matches v1's PASSED/SKIPPED weighting
			outcome = "SKIPPED"
		} else {
			passed++
		}

		lines = append(lines, fmt.Sprintf("%s::%s %s [%3d%%]", file, name, outcome, pct))
	}

	return lines, passed
}
