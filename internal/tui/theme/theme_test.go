package theme

import (
	"sort"
	"testing"
)

func TestGetReturnsDefaultTheme(t *testing.T) {
	th := Get("opencode-mono")
	if th.Name != "opencode-mono" {
		t.Fatalf("expected opencode-mono, got %q", th.Name)
	}
}

func TestGetReturnEachThemeByName(t *testing.T) {
	for _, name := range []string{"opencode-mono", "github-dark", "warm-minimal"} {
		th := Get(name)
		if th.Name != name {
			t.Errorf("Get(%q) returned theme named %q", name, th.Name)
		}
	}
}

func TestGetFallsBackToDefault(t *testing.T) {
	th := Get("nonexistent-theme")
	if th.Name != "opencode-mono" {
		t.Fatalf("expected fallback to opencode-mono, got %q", th.Name)
	}
}

func TestListReturnsThreeThemes(t *testing.T) {
	names := List()
	if len(names) != 3 {
		t.Fatalf("expected 3 themes, got %d: %v", len(names), names)
	}

	sort.Strings(names)
	expected := []string{"github-dark", "opencode-mono", "warm-minimal"}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("expected %q at index %d, got %q", expected[i], i, n)
		}
	}
}

func TestDefaultReturnsOpenCodeMono(t *testing.T) {
	if Default() != "opencode-mono" {
		t.Fatalf("expected default to be opencode-mono, got %q", Default())
	}
}

func TestStylesReturnsNonZeroStyles(t *testing.T) {
	for _, name := range []string{"opencode-mono", "github-dark", "warm-minimal"} {
		th := Get(name)
		s := th.Styles()

		// Verify that the styles are not the zero value by checking that
		// rendering them produces non-empty output or that they have been
		// configured. We test a representative sample.
		if s.Title.Render("x") == "" {
			t.Errorf("theme %q: Title.Render produced empty string", name)
		}
		if s.StatusBar.Render("x") == "" {
			t.Errorf("theme %q: StatusBar.Render produced empty string", name)
		}
		if s.Error.Render("x") == "" {
			t.Errorf("theme %q: Error.Render produced empty string", name)
		}
	}
}
