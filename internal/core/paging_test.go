package core

import "testing"

func sr(ids ...string) []SearchResult {
	out := make([]SearchResult, len(ids))
	for i, id := range ids {
		out[i] = SearchResult{VideoID: id, Title: "t-" + id}
	}
	return out
}

func TestMergeResults(t *testing.T) {
	merged, added := MergeResults(sr("a", "b"), sr("b", "c", "d"))
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}
	if len(merged) != 4 {
		t.Fatalf("len = %d, want 4", len(merged))
	}
	want := []string{"a", "b", "c", "d"}
	for i, w := range want {
		if merged[i].VideoID != w {
			t.Errorf("merged[%d] = %s, want %s", i, merged[i].VideoID, w)
		}
	}
}

func TestMergeResultsAllDuplicates(t *testing.T) {
	merged, added := MergeResults(sr("a", "b"), sr("a", "b"))
	if added != 0 || len(merged) != 2 {
		t.Errorf("added=%d len=%d, want 0 and 2", added, len(merged))
	}
}

func TestMergeResultsEmptyExisting(t *testing.T) {
	merged, added := MergeResults(nil, sr("a"))
	if added != 1 || len(merged) != 1 {
		t.Errorf("added=%d len=%d, want 1 and 1", added, len(merged))
	}
}

func TestNextFetchSize(t *testing.T) {
	cases := []struct{ cur, batch, max, want int }{
		{10, 25, 100, 35},
		{85, 25, 100, 100}, // clamped to cap
		{100, 25, 100, 0},  // at cap
		{120, 25, 100, 0},  // beyond cap
		{0, 25, 100, 25},
	}
	for _, c := range cases {
		if got := NextFetchSize(c.cur, c.batch, c.max); got != c.want {
			t.Errorf("NextFetchSize(%d,%d,%d) = %d, want %d", c.cur, c.batch, c.max, got, c.want)
		}
	}
}

func TestVisibleRange(t *testing.T) {
	cases := []struct{ cursor, total, height, wantStart, wantEnd int }{
		{0, 5, 10, 0, 5},       // everything fits
		{0, 100, 10, 0, 10},    // top
		{50, 100, 10, 45, 55},  // centered on cursor
		{99, 100, 10, 90, 100}, // bottom clamp
		{3, 100, 10, 0, 10},    // near top clamp
		{0, 0, 10, 0, 0},       // empty
		{0, 5, 0, 0, 0},        // degenerate height
	}
	for _, c := range cases {
		s, e := VisibleRange(c.cursor, c.total, c.height)
		if s != c.wantStart || e != c.wantEnd {
			t.Errorf("VisibleRange(%d,%d,%d) = [%d,%d), want [%d,%d)",
				c.cursor, c.total, c.height, s, e, c.wantStart, c.wantEnd)
		}
	}
}
