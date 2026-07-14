package core

import "time"

// RemoteCommand identifies a hardware/OS media-key or media-session control
// event delivered asynchronously by a MediaRemote adapter (e.g. a keyboard
// play/pause key, a notification-widget button, or a Bluetooth headset
// control).
type RemoteCommand int

const (
	RemotePlayPause RemoteCommand = iota
	RemoteNext
	RemotePrev
	RemoteStop
)

// NowPlaying is the metadata/state snapshot the TUI publishes to the OS
// media session (e.g. MPRIS on Linux, SMTC on Windows) so hardware media
// keys and lock-screen/notification widgets reflect the current track.
type NowPlaying struct {
	Title    string
	Artist   string
	Duration time.Duration
	Position time.Duration
	Playing  bool
}

// MediaRemote is the port through which the TUI receives hardware media-key
// commands and publishes now-playing state to the OS. Platform adapters
// (Linux MPRIS, Windows SMTC) and the noop fallback all implement this; the
// TUI depends only on this interface, never on a concrete adapter.
//
// Commands returns a channel of inbound remote commands. Adapters MUST
// buffer this channel (recommended capacity: 8) and MUST NOT block their
// OS-facing event loop if the TUI is slow to drain it — when the buffer is
// full, the adapter drops the new command rather than blocking.
type MediaRemote interface {
	// Commands returns a receive-only channel of inbound remote commands.
	// Well-behaved adapters never close it while running; callers should
	// stop reading once Close has been called.
	Commands() <-chan RemoteCommand

	// Publish pushes the latest now-playing state to the OS media session.
	// Implementations must not block the caller for long; best-effort only.
	Publish(np NowPlaying)

	// Close releases any OS resources held by the adapter (D-Bus
	// connections, handles, etc). Safe to call multiple times.
	Close() error
}
