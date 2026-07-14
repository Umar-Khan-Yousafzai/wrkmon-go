package mediakeys

import (
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/godbus/dbus/v5"
)

func TestPlaybackStatus(t *testing.T) {
	cases := []struct {
		name string
		np   core.NowPlaying
		want string
	}{
		{"playing with title", core.NowPlaying{Title: "Song", Playing: true}, "Playing"},
		{"playing without title", core.NowPlaying{Playing: true}, "Playing"},
		{"paused with title", core.NowPlaying{Title: "Song", Playing: false}, "Paused"},
		{"stopped no title", core.NowPlaying{Title: "", Playing: false}, "Stopped"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := playbackStatus(tc.np); got != tc.want {
				t.Fatalf("playbackStatus(%+v) = %q, want %q", tc.np, got, tc.want)
			}
		})
	}
}

func TestMPRISMetadata(t *testing.T) {
	np := core.NowPlaying{
		Title:    "Never Gonna Give You Up",
		Artist:   "Rick Astley",
		Duration: 3*time.Minute + 33*time.Second,
		Position: 42 * time.Second,
		Playing:  true,
	}
	m := mprisMetadata(np)

	// xesam:title -> string
	titleV, ok := m["xesam:title"]
	if !ok {
		t.Fatal("missing xesam:title")
	}
	if got, ok := titleV.Value().(string); !ok || got != np.Title {
		t.Fatalf("xesam:title = %#v, want string %q", titleV.Value(), np.Title)
	}

	// xesam:artist -> []string
	artistV, ok := m["xesam:artist"]
	if !ok {
		t.Fatal("missing xesam:artist")
	}
	artists, ok := artistV.Value().([]string)
	if !ok {
		t.Fatalf("xesam:artist type = %T, want []string", artistV.Value())
	}
	if len(artists) != 1 || artists[0] != np.Artist {
		t.Fatalf("xesam:artist = %#v, want [%q]", artists, np.Artist)
	}

	// mpris:length -> int64 microseconds
	lenV, ok := m["mpris:length"]
	if !ok {
		t.Fatal("missing mpris:length")
	}
	gotLen, ok := lenV.Value().(int64)
	if !ok {
		t.Fatalf("mpris:length type = %T, want int64", lenV.Value())
	}
	if wantLen := np.Duration.Microseconds(); gotLen != wantLen {
		t.Fatalf("mpris:length = %d, want %d", gotLen, wantLen)
	}

	// mpris:trackid -> dbus.ObjectPath
	tidV, ok := m["mpris:trackid"]
	if !ok {
		t.Fatal("missing mpris:trackid")
	}
	tid, ok := tidV.Value().(dbus.ObjectPath)
	if !ok {
		t.Fatalf("mpris:trackid type = %T, want dbus.ObjectPath", tidV.Value())
	}
	if tid != dbus.ObjectPath("/org/wrkmon/track/1") {
		t.Fatalf("mpris:trackid = %q, want /org/wrkmon/track/1", tid)
	}
}
