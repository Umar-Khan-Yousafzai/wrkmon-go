package mediakeys

import "github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"

// Windows virtual-key codes for the four dedicated media keys. These double
// as RegisterHotKey ids on Windows (id == vk keeps the WM_HOTKEY -> command
// lookup a single map access). This file carries no build constraint so the
// id -> core.RemoteCommand mapping is unit-testable on every platform, even
// though only hotkeys_windows.go ever calls RegisterHotKey with them.
const (
	vkMediaNext      = 0xB0
	vkMediaPrev      = 0xB1
	vkMediaStop      = 0xB2
	vkMediaPlayPause = 0xB3
)

// hotkeyCommands maps a Windows virtual-key code (used as both the
// RegisterHotKey id and vk parameter) to the core.RemoteCommand it
// represents.
var hotkeyCommands = map[int]core.RemoteCommand{
	vkMediaPlayPause: core.RemotePlayPause,
	vkMediaNext:      core.RemoteNext,
	vkMediaPrev:      core.RemotePrev,
	vkMediaStop:      core.RemoteStop,
}
