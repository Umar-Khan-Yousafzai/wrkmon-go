package mpv

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/ports"
)

// Compile-time check: MPV must satisfy ports.Player.
var _ ports.Player = (*MPV)(nil)

// MPV controls an mpv subprocess via JSON IPC.
type MPV struct {
	cmd      *exec.Cmd
	conn     net.Conn
	reader   *bufio.Reader
	sockPath string
	mu       sync.Mutex
	reqID    atomic.Int64
	running  atomic.Bool
	lastURL  string // stored for respawn on crash
	lastVol  int    // stored for respawn on crash
}

// ipcRequest is the JSON-IPC command format mpv expects.
type ipcRequest struct {
	Command   []interface{} `json:"command"`
	RequestID int64         `json:"request_id"`
}

// ipcResponse is the JSON-IPC response format mpv sends back.
type ipcResponse struct {
	Data      interface{} `json:"data"`
	RequestID int64       `json:"request_id"`
	Error     string      `json:"error"`
}

// New creates an MPV adapter. No subprocess is started until Play is called.
func New() *MPV {
	return &MPV{}
}

// Play stops any existing mpv instance, then launches a new one for the given
// audio URL and connects to its IPC socket.
func (m *MPV) Play(url string) error {
	m.stop()

	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("wrkmon-mpv-%d.sock", os.Getpid()))
	os.Remove(sockPath)

	cmd := exec.Command("mpv",
		"--no-video",
		"--no-terminal",
		fmt.Sprintf("--input-ipc-server=%s", sockPath),
		url,
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mpv: %w", err)
	}

	m.mu.Lock()
	m.cmd = cmd
	m.sockPath = sockPath
	m.mu.Unlock()
	m.running.Store(true)

	// Poll until the IPC socket appears and accepts a connection.
	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ { // up to 5 seconds
		time.Sleep(100 * time.Millisecond)
		conn, err = dialSocket(sockPath)
		if err == nil {
			break
		}
	}
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		m.running.Store(false)
		return fmt.Errorf("connect to mpv socket: %w", err)
	}

	m.mu.Lock()
	m.conn = conn
	m.reader = bufio.NewReader(conn)
	m.lastURL = url
	m.mu.Unlock()

	// Watch for process exit in the background.
	go func() {
		cmd.Wait()
		m.running.Store(false)
	}()

	return nil
}

// Respawn attempts to restart mpv with the last played URL.
// Returns an error if no URL was previously played or if respawn fails.
func (m *MPV) Respawn() error {
	m.mu.Lock()
	url := m.lastURL
	vol := m.lastVol
	m.mu.Unlock()

	if url == "" {
		return fmt.Errorf("no previous URL to respawn")
	}

	if err := m.Play(url); err != nil {
		return fmt.Errorf("respawn failed: %w", err)
	}

	// Restore volume
	if vol > 0 {
		m.command("set_property", "volume", float64(vol))
	}

	return nil
}

// Pause pauses playback.
func (m *MPV) Pause() error {
	_, err := m.command("set_property", "pause", true)
	return err
}

// Resume resumes playback after a pause.
func (m *MPV) Resume() error {
	_, err := m.command("set_property", "pause", false)
	return err
}

// Stop kills the mpv process and cleans up resources.
func (m *MPV) Stop() error {
	m.stop()
	return nil
}

// Seek seeks relative to the current position by the given number of seconds.
// Positive values seek forward, negative values seek backward.
func (m *MPV) Seek(seconds float64) error {
	_, err := m.command("seek", seconds, "relative")
	return err
}

// SetVolume sets the playback volume (0–100).
func (m *MPV) SetVolume(vol int) error {
	_, err := m.command("set_property", "volume", float64(vol))
	if err == nil {
		m.mu.Lock()
		m.lastVol = vol
		m.mu.Unlock()
	}
	return err
}

// GetPosition returns the current playback position in seconds.
func (m *MPV) GetPosition() (float64, error) {
	resp, err := m.command("get_property", "time-pos")
	if err != nil {
		return 0, err
	}
	switch v := resp.Data.(type) {
	case float64:
		return v, nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("unexpected position type %T", resp.Data)
	}
}

// GetDuration returns the total duration of the current track in seconds.
func (m *MPV) GetDuration() (float64, error) {
	resp, err := m.command("get_property", "duration")
	if err != nil {
		return 0, err
	}
	switch v := resp.Data.(type) {
	case float64:
		return v, nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("unexpected duration type %T", resp.Data)
	}
}

// IsRunning reports whether the mpv process is still alive.
func (m *MPV) IsRunning() bool {
	return m.running.Load()
}

// Close stops playback and releases all resources.
func (m *MPV) Close() error {
	m.stop()
	return nil
}

// command sends a JSON IPC command to mpv and waits for the matching response.
func (m *MPV) command(args ...interface{}) (*ipcResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	id := m.reqID.Add(1)
	req := ipcRequest{
		Command:   args,
		RequestID: id,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}
	data = append(data, '\n')

	m.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := m.conn.Write(data); err != nil {
		return nil, fmt.Errorf("write to mpv: %w", err)
	}

	// Read response lines, skipping unsolicited events (request_id == 0),
	// until we find the response matching our request_id.
	for {
		m.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := m.reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("read from mpv: %w", err)
		}
		var resp ipcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // skip malformed lines
		}
		if resp.RequestID == id {
			if resp.Error != "" && resp.Error != "success" {
				return nil, fmt.Errorf("mpv: %s", resp.Error)
			}
			return &resp, nil
		}
		// Not our response — keep reading (likely an event).
	}
}

// stop shuts down the mpv subprocess and cleans up the IPC connection and socket.
func (m *MPV) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
		m.reader = nil
	}

	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
		m.cmd = nil
	}

	if m.sockPath != "" {
		os.Remove(m.sockPath)
		m.sockPath = ""
	}

	m.running.Store(false)
}
