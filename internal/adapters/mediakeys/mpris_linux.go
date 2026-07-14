//go:build linux

package mediakeys

import (
	"fmt"
	"sync"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// MPRIS binding constants. The bus-name suffix must be a valid D-Bus name
// element (no hyphens), so it is "wrkmon_go" regardless of the app's display
// name; the human-facing name is the Identity property ("wrkmon").
const (
	mprisBusName     = "org.mpris.MediaPlayer2.wrkmon_go"
	mprisPath        = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	mprisRootIface   = "org.mpris.MediaPlayer2"
	mprisPlayerIface = "org.mpris.MediaPlayer2.Player"
)

// mprisRemote is the Linux core.MediaRemote adapter. It owns a D-Bus session
// connection, exports the MPRIS root + Player interfaces, translates inbound
// MPRIS method calls into core.RemoteCommand values, and publishes now-playing
// state as MPRIS properties.
type mprisRemote struct {
	conn     *dbus.Conn
	props    *prop.Properties
	commands chan core.RemoteCommand

	mu         sync.Mutex
	closed     bool
	hasLast    bool
	lastStatus string
	lastTitle  string
}

// newMPRIS constructs the Linux MPRIS adapter. On any bus/registration error
// it returns an error (and leaves no bus name claimed); the caller falls back
// to Noop. appName is accepted for interface symmetry with other platforms but
// is not used: MPRIS identity is fixed ("wrkmon") and the bus name is fixed.
func newMPRIS(appName string) (core.MediaRemote, error) {
	_ = appName

	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("mpris: connect session bus: %w", err)
	}

	r := &mprisRemote{
		conn:     conn,
		commands: make(chan core.RemoteCommand, 8),
	}

	// Export methods. A method call is dispatched by (path, iface, method),
	// so the root and Player method tables are independent even though both
	// live on the same object path.
	// Table keys are the MPRIS D-Bus method names; the Go handler names are
	// deliberately unexported ("on*") so they are decoupled from the wire
	// names (and so "Seek" doesn't collide with go vet's io.Seeker check).
	rootMethods := map[string]any{
		"Raise": r.onRaise,
		"Quit":  r.onQuit,
	}
	if err := conn.ExportMethodTable(rootMethods, mprisPath, mprisRootIface); err != nil {
		return nil, closeOnErr(conn, fmt.Errorf("mpris: export root iface: %w", err))
	}
	playerMethods := map[string]any{
		"Next":        r.onNext,
		"Previous":    r.onPrevious,
		"Pause":       r.onPause,
		"PlayPause":   r.onPlayPause,
		"Stop":        r.onStop,
		"Play":        r.onPlay,
		"Seek":        r.onSeek,
		"SetPosition": r.onSetPosition,
		"OpenUri":     r.onOpenURI,
	}
	if err := conn.ExportMethodTable(playerMethods, mprisPath, mprisPlayerIface); err != nil {
		return nil, closeOnErr(conn, fmt.Errorf("mpris: export player iface: %w", err))
	}

	// Export properties (also installs org.freedesktop.DBus.Properties).
	props, err := prop.Export(conn, mprisPath, mprisPropSpec())
	if err != nil {
		return nil, closeOnErr(conn, fmt.Errorf("mpris: export properties: %w", err))
	}
	r.props = props

	// Export introspection so clients that introspect (some do before
	// GetAll) see the interfaces, methods and properties.
	node := mprisIntrospectNode(props)
	if err := conn.Export(introspect.NewIntrospectable(node), mprisPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, closeOnErr(conn, fmt.Errorf("mpris: export introspectable: %w", err))
	}

	// Claim the well-known bus name. DoNotQueue: if another player already
	// owns it we fail immediately rather than waiting in the queue.
	reply, err := conn.RequestName(mprisBusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, closeOnErr(conn, fmt.Errorf("mpris: request name: %w", err))
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, closeOnErr(conn, fmt.Errorf("mpris: bus name %q not available (reply %d)", mprisBusName, reply))
	}

	return r, nil
}

// closeOnErr closes conn (best-effort) and returns err, so constructor error
// paths release the connection without leaking it.
func closeOnErr(conn *dbus.Conn, err error) error {
	_ = conn.Close()
	return err
}

// mprisPropSpec builds the MPRIS property map. PlaybackStatus and Metadata
// emit PropertiesChanged when set; Position never signals (per MPRIS spec);
// the constant capability/identity properties never change.
func mprisPropSpec() prop.Map {
	empty := core.NowPlaying{}
	return prop.Map{
		mprisRootIface: {
			"CanQuit":             {Value: false, Emit: prop.EmitConst},
			"CanRaise":            {Value: false, Emit: prop.EmitConst},
			"Identity":            {Value: "wrkmon", Emit: prop.EmitConst},
			"SupportedUriSchemes": {Value: []string{}, Emit: prop.EmitConst},
			"SupportedMimeTypes":  {Value: []string{}, Emit: prop.EmitConst},
		},
		mprisPlayerIface: {
			"PlaybackStatus": {Value: playbackStatus(empty), Emit: prop.EmitTrue},
			"Metadata":       {Value: mprisMetadata(empty), Emit: prop.EmitTrue},
			"Position":       {Value: int64(0), Emit: prop.EmitFalse},
			"Rate":           {Value: 1.0, Emit: prop.EmitConst},
			"MinimumRate":    {Value: 1.0, Emit: prop.EmitConst},
			"MaximumRate":    {Value: 1.0, Emit: prop.EmitConst},
			"Volume":         {Value: 1.0, Emit: prop.EmitConst},
			"CanGoNext":      {Value: true, Emit: prop.EmitConst},
			"CanGoPrevious":  {Value: true, Emit: prop.EmitConst},
			"CanPlay":        {Value: true, Emit: prop.EmitConst},
			"CanPause":       {Value: true, Emit: prop.EmitConst},
			"CanSeek":        {Value: false, Emit: prop.EmitConst},
			"CanControl":     {Value: true, Emit: prop.EmitConst},
		},
	}
}

// mprisIntrospectNode assembles the introspection tree for the object,
// including the property definitions from p.
func mprisIntrospectNode(p *prop.Properties) *introspect.Node {
	noArg := func(name string) introspect.Method { return introspect.Method{Name: name} }
	return &introspect.Node{
		Name: string(mprisPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:       mprisRootIface,
				Methods:    []introspect.Method{noArg("Raise"), noArg("Quit")},
				Properties: p.Introspection(mprisRootIface),
			},
			{
				Name: mprisPlayerIface,
				Methods: []introspect.Method{
					noArg("Next"), noArg("Previous"), noArg("Pause"),
					noArg("PlayPause"), noArg("Stop"), noArg("Play"),
					{Name: "Seek", Args: []introspect.Arg{{Name: "Offset", Type: "x", Direction: "in"}}},
					{Name: "SetPosition", Args: []introspect.Arg{
						{Name: "TrackId", Type: "o", Direction: "in"},
						{Name: "Position", Type: "x", Direction: "in"},
					}},
					{Name: "OpenUri", Args: []introspect.Arg{{Name: "Uri", Type: "s", Direction: "in"}}},
				},
				Signals: []introspect.Signal{
					{Name: "Seeked", Args: []introspect.Arg{{Name: "Position", Type: "x"}}},
				},
				Properties: p.Introspection(mprisPlayerIface),
			},
		},
	}
}

// push enqueues cmd, dropping it if the buffer is full. It never blocks, so a
// slow TUI can never stall a D-Bus method handler.
func (r *mprisRemote) push(cmd core.RemoteCommand) {
	select {
	case r.commands <- cmd:
	default:
	}
}

// ---- org.mpris.MediaPlayer2 (root) method handlers ----

// onRaise handles Raise: a no-op (CanRaise is false; no window to raise).
func (r *mprisRemote) onRaise() *dbus.Error { return nil }

// onQuit handles Quit: a no-op (CanQuit is false; the media session must not
// kill the TUI).
func (r *mprisRemote) onQuit() *dbus.Error { return nil }

// ---- org.mpris.MediaPlayer2.Player method handlers ----

func (r *mprisRemote) onNext() *dbus.Error      { r.push(core.RemoteNext); return nil }
func (r *mprisRemote) onPrevious() *dbus.Error  { r.push(core.RemotePrev); return nil }
func (r *mprisRemote) onPause() *dbus.Error     { r.push(core.RemotePlayPause); return nil }
func (r *mprisRemote) onPlayPause() *dbus.Error { r.push(core.RemotePlayPause); return nil }
func (r *mprisRemote) onPlay() *dbus.Error      { r.push(core.RemotePlayPause); return nil }
func (r *mprisRemote) onStop() *dbus.Error      { r.push(core.RemoteStop); return nil }

// onSeek handles Seek: a no-op in v1 (CanSeek is false).
func (r *mprisRemote) onSeek(offset int64) *dbus.Error { return nil }

// onSetPosition handles SetPosition: a no-op in v1 (CanSeek is false).
func (r *mprisRemote) onSetPosition(trackID dbus.ObjectPath, position int64) *dbus.Error {
	return nil
}

// onOpenURI handles OpenUri: a no-op (wrkmon-go accepts no external URIs).
func (r *mprisRemote) onOpenURI(uri string) *dbus.Error { return nil }

// ---- core.MediaRemote implementation ----

// Commands returns the buffered channel of inbound remote commands.
func (r *mprisRemote) Commands() <-chan core.RemoteCommand { return r.commands }

// Publish updates the MPRIS properties from np. PlaybackStatus and Metadata
// are only re-set (and thus only signal PropertiesChanged) when the
// playing-state or title actually changed since the last publish, avoiding a
// signal storm from per-tick position updates. Position is always updated but
// never signalled (per MPRIS spec).
func (r *mprisRemote) Publish(np core.NowPlaying) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	status := playbackStatus(np)
	changed := !r.hasLast || status != r.lastStatus || np.Title != r.lastTitle
	r.hasLast = true
	r.lastStatus = status
	r.lastTitle = np.Title
	props := r.props
	r.mu.Unlock()

	if props == nil {
		return
	}
	if changed {
		props.SetMust(mprisPlayerIface, "PlaybackStatus", status)
		props.SetMust(mprisPlayerIface, "Metadata", mprisMetadata(np))
	}
	props.SetMust(mprisPlayerIface, "Position", np.Position.Microseconds())
}

// Close releases the bus name and connection. It is idempotent: the guard
// makes repeated calls safe no-ops. The commands channel is deliberately not
// closed (its documented contract) — after the connection closes, no handler
// can push again.
func (r *mprisRemote) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	conn := r.conn
	r.mu.Unlock()

	if conn == nil {
		return nil
	}
	_, _ = conn.ReleaseName(mprisBusName)
	return conn.Close()
}
