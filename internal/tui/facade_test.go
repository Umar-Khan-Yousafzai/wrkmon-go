package tui

import (
	"context"
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

type fakeSearcher struct {
	lastLimit int
	results   []core.SearchResult
}

func (f *fakeSearcher) Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	f.lastLimit = limit
	return f.results, nil
}
func (f *fakeSearcher) GetStreamURL(ctx context.Context, videoID string) (string, error) {
	return "http://stream/" + videoID, nil
}

type stubStore struct {
	cached      []core.SearchResult
	lastCached  []core.SearchResult
	cachedQuery string
}

func (s *stubStore) SaveHistory(ctx context.Context, e core.HistoryEntry) error { return nil }
func (s *stubStore) GetHistory(ctx context.Context, l, o int) ([]core.HistoryEntry, error) {
	return nil, nil
}
func (s *stubStore) SearchHistory(ctx context.Context, q string, l int) ([]core.HistoryEntry, error) {
	return nil, nil
}
func (s *stubStore) SaveQueue(ctx context.Context, t []core.Track, c int) error { return nil }
func (s *stubStore) LoadQueue(ctx context.Context) ([]core.Track, int, error)   { return nil, 0, nil }
func (s *stubStore) CacheSearchResults(ctx context.Context, q string, r []core.SearchResult, ttl time.Duration) error {
	s.cachedQuery, s.lastCached = q, r
	return nil
}
func (s *stubStore) GetCachedSearch(ctx context.Context, q string) ([]core.SearchResult, bool, error) {
	return s.cached, len(s.cached) > 0, nil
}
func (s *stubStore) CacheLyrics(ctx context.Context, v, l string) error { return nil }
func (s *stubStore) GetCachedLyrics(ctx context.Context, v string) (string, bool, error) {
	return "", false, nil
}
func (s *stubStore) SaveDownload(ctx context.Context, d core.Download) error { return nil }
func (s *stubStore) ListDownloads(ctx context.Context, l int) ([]core.Download, error) {
	return nil, nil
}
func (s *stubStore) CreatePlaylist(ctx context.Context, n string) (core.Playlist, error) {
	return core.Playlist{}, nil
}
func (s *stubStore) ListPlaylists(ctx context.Context) ([]core.Playlist, error) { return nil, nil }
func (s *stubStore) GetPlaylist(ctx context.Context, id int) (core.Playlist, error) {
	return core.Playlist{}, nil
}
func (s *stubStore) DeletePlaylist(ctx context.Context, id int) error              { return nil }
func (s *stubStore) AddToPlaylist(ctx context.Context, id int, t core.Track) error { return nil }
func (s *stubStore) RemoveFromPlaylist(ctx context.Context, id, pos int) error     { return nil }
func (s *stubStore) Close() error                                                  { return nil }

func nResults(n int) []core.SearchResult {
	out := make([]core.SearchResult, n)
	for i := range out {
		out[i] = core.SearchResult{VideoID: string(rune('a' + i))}
	}
	return out
}

func TestSearchCacheHitNotTruncated(t *testing.T) {
	st := &stubStore{cached: nResults(30)}
	f := NewFacade(&fakeSearcher{}, nil, st)
	got, err := f.Search(context.Background(), "q", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 30 {
		t.Errorf("cache hit returned %d results, want the full 30", len(got))
	}
}

func TestSearchMoreBypassesCache(t *testing.T) {
	se := &fakeSearcher{results: nResults(5)}
	st := &stubStore{cached: nResults(30)} // cache would hit — must be ignored
	f := NewFacade(se, nil, st)
	got, err := f.SearchMore(context.Background(), "q", 35)
	if err != nil {
		t.Fatal(err)
	}
	if se.lastLimit != 35 {
		t.Errorf("searcher got limit %d, want 35", se.lastLimit)
	}
	if len(got) != 5 {
		t.Errorf("got %d results, want the searcher's 5", len(got))
	}
}

func TestCacheSearchOverwrites(t *testing.T) {
	st := &stubStore{}
	f := NewFacade(&fakeSearcher{}, nil, st)
	f.CacheSearch(context.Background(), "q", nResults(12))
	if st.cachedQuery != "q" || len(st.lastCached) != 12 {
		t.Errorf("cached %q/%d, want q/12", st.cachedQuery, len(st.lastCached))
	}
}
