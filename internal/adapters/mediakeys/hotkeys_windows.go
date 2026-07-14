//go:build windows

package mediakeys

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"golang.org/x/sys/windows"
)

// user32 procs used for global hotkey registration and the thread message
// loop. RegisterHotKey/UnregisterHotKey/GetMessageW/PostThreadMessageW have
// no ANSI/Unicode split among these names (only GetMessage takes a string
// arg in some Win32 APIs, and it doesn't here), so the plain and "W" names
// are used as documented by MSDN.
var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
)

const (
	wmHotkey = 0x0312
	wmQuit   = 0x0012
)

// winMsg mirrors the layout of the Win32 MSG struct so GetMessageW can fill
// it in via a raw syscall:
//
//	typedef struct tagMSG {
//	  HWND   hwnd;
//	  UINT   message;
//	  WPARAM wParam;
//	  LPARAM lParam;
//	  DWORD  time;
//	  POINT  pt;
//	} MSG;
//
// The pointer-sized fields (hwnd, wParam, lParam) naturally align on 8-byte
// boundaries on amd64, matching the real struct's padding after the 4-byte
// message field.
type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

// hotkeysRemote is the Windows core.MediaRemote adapter. It owns a
// dedicated OS thread that registers global hotkeys for the four media keys
// via RegisterHotKey and pumps a GetMessageW loop, translating WM_HOTKEY
// messages into core.RemoteCommand values delivered over Commands().
type hotkeysRemote struct {
	commands chan core.RemoteCommand

	mu       sync.Mutex
	closed   bool
	threadID uint32
}

// newHotkeys constructs the Windows adapter. It starts the message-loop
// goroutine and blocks until that goroutine has captured its OS thread id
// and attempted hotkey registration, so by the time newHotkeys returns,
// Close is always safe to call (it needs threadID to post WM_QUIT).
// Registration itself is best-effort: partial or total registration
// failure is logged to stderr but never turns into a constructor error,
// since a hotkey adapter with zero working keys still safely satisfies
// core.MediaRemote (Commands() just never fires).
func newHotkeys() (core.MediaRemote, error) {
	r := &hotkeysRemote{
		commands: make(chan core.RemoteCommand, 8),
	}

	ready := make(chan struct{})
	go r.run(ready)
	<-ready

	return r, nil
}

// run is the body of the dedicated OS thread. RegisterHotKey,
// UnregisterHotKey and the thread-message queue drained by GetMessageW are
// all thread-affine in Win32, so this goroutine must stay locked to one OS
// thread for its entire lifetime: it registers the hotkeys, signals ready,
// pumps messages until WM_QUIT, then unregisters on its way out (on the
// same thread that registered, as required).
func (r *hotkeysRemote) run(ready chan<- struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	r.mu.Lock()
	r.threadID = windows.GetCurrentThreadId()
	r.mu.Unlock()

	registered := make(map[int]bool, len(hotkeyCommands))
	for vk := range hotkeyCommands {
		// RegisterHotKey(hWnd=0, id=vk, fsModifiers=0, vk). hWnd=0 associates
		// the hotkey with this thread's message queue rather than a window,
		// which is exactly what a headless TUI needs.
		ok, _, callErr := procRegisterHotKey.Call(0, uintptr(vk), 0, uintptr(vk))
		if ok == 0 {
			fmt.Fprintf(os.Stderr, "wrkmon: media hotkey 0x%02X registration failed: %v\n", vk, callErr)
			continue
		}
		registered[vk] = true
	}

	close(ready)

	var m winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		// GetMessageW returns 0 on WM_QUIT and -1 on error; both end the
		// loop. -1 as the BOOL (32-bit) return value only reliably shows up
		// in the low 32 bits of the raw syscall return, so truncate before
		// comparing rather than assuming upper-bit sign extension.
		if ret == 0 || int32(ret) == -1 {
			break
		}
		if m.message == wmHotkey {
			if cmd, ok := hotkeyCommands[int(m.wParam)]; ok {
				r.push(cmd)
			}
		}
	}

	for vk := range registered {
		procUnregisterHotKey.Call(0, uintptr(vk))
	}
}

// push enqueues cmd, dropping it if the buffer is full. It never blocks, so
// a slow TUI can never stall the Win32 message-loop thread.
func (r *hotkeysRemote) push(cmd core.RemoteCommand) {
	select {
	case r.commands <- cmd:
	default:
	}
}

// Commands returns the buffered channel of inbound remote commands.
func (r *hotkeysRemote) Commands() <-chan core.RemoteCommand { return r.commands }

// Publish is a no-op: this adapter only covers RegisterHotKey-based media
// keys, not Windows' System Media Transport Controls (SMTC) now-playing
// widget, so there's no OS-facing state to push.
func (r *hotkeysRemote) Publish(core.NowPlaying) {}

// Close posts WM_QUIT to the message-loop thread so GetMessageW returns and
// the loop's own UnregisterHotKey cleanup (run on the correct thread) runs.
// Idempotent: repeated calls are safe no-ops.
func (r *hotkeysRemote) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	tid := r.threadID
	r.mu.Unlock()

	procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0)
	return nil
}
