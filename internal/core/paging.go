package core

// MergeResults appends items of incoming whose VideoID is not already in
// existing, preserving order. Returns the merged slice and the count added.
func MergeResults(existing, incoming []SearchResult) ([]SearchResult, int) {
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		seen[r.VideoID] = struct{}{}
	}
	added := 0
	for _, r := range incoming {
		if _, dup := seen[r.VideoID]; dup {
			continue
		}
		seen[r.VideoID] = struct{}{}
		existing = append(existing, r)
		added++
	}
	return existing, added
}

// NextFetchSize returns the total to request from yt-dlp for the next
// infinite-scroll fetch, or 0 when current has reached max.
func NextFetchSize(current, batch, max int) int {
	if current >= max {
		return 0
	}
	if n := current + batch; n < max {
		return n
	}
	return max
}

// VisibleRange returns the half-open row window [start, end) that keeps
// cursor visible (roughly centered) in a viewport of the given height.
func VisibleRange(cursor, total, height int) (int, int) {
	if height <= 0 || total <= 0 {
		return 0, 0
	}
	if total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}
