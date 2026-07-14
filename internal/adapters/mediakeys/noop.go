package mediakeys

import "github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"

// Noop is a core.MediaRemote that never delivers commands and discards
// published state. It's the fallback for platforms without a real adapter
// yet, and the constructed value when the user disables media_keys in
// config.
type Noop struct {
	// ch is always nil (Noop{} is used directly, never via a constructor
	// that would allocate one) and is never closed. Commands() returns it
	// as-is: a typed nil channel, safe to read from (blocks forever, never
	// panics) and safe to range over (never iterates).
	ch chan core.RemoteCommand
}

// Commands returns a nil channel: reads block forever and never fire.
func (n Noop) Commands() <-chan core.RemoteCommand { return n.ch }

// Publish discards the now-playing snapshot.
func (Noop) Publish(core.NowPlaying) {}

// Close is a no-op.
func (Noop) Close() error { return nil }
