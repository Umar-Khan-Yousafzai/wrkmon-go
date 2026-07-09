package window

import (
	"runtime"
	"strings"
	"testing"
)

func fakeLook(installed ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, b := range installed {
		set[b] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", &notFoundError{name}
	}
}

type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return e.name + " not found" }

func TestResolveAutoPicksFirstInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	bin, args, err := Resolve("/opt/wrkmon-go", "auto", nil, fakeLook("alacritty", "xterm"))
	if err != nil {
		t.Fatal(err)
	}
	if bin != "/usr/bin/alacritty" {
		t.Errorf("bin = %s, want alacritty (first installed in priority order)", bin)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "wrkmon-go") || !strings.Contains(joined, "/opt/wrkmon-go") {
		t.Errorf("args missing class or self path: %v", args)
	}
}

func TestResolveKittyArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	_, args, err := Resolve("/opt/wrkmon-go", "auto", nil, fakeLook("kitty"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--class=wrkmon-go", "--title=wrkmon", "-e", "/opt/wrkmon-go"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %s, want %s", i, args[i], want[i])
		}
	}
}

func TestResolveOverrideHonored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	bin, _, err := Resolve("/opt/wrkmon-go", "foot", nil, fakeLook("kitty", "foot"))
	if err != nil {
		t.Fatal(err)
	}
	if bin != "/usr/bin/foot" {
		t.Errorf("bin = %s, want foot (override)", bin)
	}
}

func TestResolveUnknownOverride(t *testing.T) {
	_, _, err := Resolve("/opt/wrkmon-go", "hyperterm", nil, fakeLook("kitty"))
	if err == nil {
		t.Error("expected error for unknown terminal name")
	}
}

func TestResolveNoneInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	_, _, err := Resolve("/opt/wrkmon-go", "auto", nil, fakeLook())
	if err == nil {
		t.Fatal("expected error when nothing installed")
	}
	if !strings.Contains(err.Error(), "kitty") {
		t.Errorf("error should list supported terminals, got: %v", err)
	}
}

// TestSpecsForWindowsWtArgv locks down the Windows Terminal ("wt") argv
// shape. It runs on any host (specsFor takes goos explicitly instead of
// reading runtime.GOOS), so it also exercises the Windows table on linux CI.
func TestSpecsForWindowsWtArgv(t *testing.T) {
	table := specsFor("windows")
	if len(table) != 1 || table[0].name != "wt" {
		t.Fatalf("windows table = %+v, want a single wt entry", table)
	}

	args := table[0].args("/opt/wrkmon-go", nil)
	want := []string{"--title", "wrkmon", "/opt/wrkmon-go"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %s, want %s", i, args[i], want[i])
		}
	}
}

// TestShouldFallback pins the gating rule: only the "probe the table" cases
// (no override, or the explicit "auto" override) may fall through to the
// OS-level conhost/Terminal.app fallback. An explicit, unresolvable override
// (e.g. a typo'd [window] terminal) must not be silently swallowed.
func TestShouldFallback(t *testing.T) {
	cases := []struct {
		override string
		want     bool
	}{
		{"", true},
		{"auto", true},
		{"kitty", false},
		{"hyperterm", false},
	}
	for _, c := range cases {
		if got := shouldFallback(c.override); got != c.want {
			t.Errorf("shouldFallback(%q) = %v, want %v", c.override, got, c.want)
		}
	}
}

func TestResolveExtraArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	_, args, err := Resolve("/opt/wrkmon-go", "kitty", []string{"--font-size=14"}, fakeLook("kitty"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "--font-size=14") {
		t.Errorf("extra args not passed through: %v", args)
	}
}
