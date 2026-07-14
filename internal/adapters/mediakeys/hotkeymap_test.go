package mediakeys

import (
	"testing"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// TestHotkeyCommands verifies the vk -> core.RemoteCommand mapping used by
// the Windows adapter. Kept in a no-build-tag file (hotkeymap.go) so this
// runs on every platform, not just Windows.
func TestHotkeyCommands(t *testing.T) {
	cases := []struct {
		name string
		vk   int
		want core.RemoteCommand
	}{
		{"play/pause", 0xB3, core.RemotePlayPause},
		{"next", 0xB0, core.RemoteNext},
		{"prev", 0xB1, core.RemotePrev},
		{"stop", 0xB2, core.RemoteStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := hotkeyCommands[tc.vk]
			if !ok {
				t.Fatalf("hotkeyCommands[0x%02X] missing", tc.vk)
			}
			if got != tc.want {
				t.Fatalf("hotkeyCommands[0x%02X] = %v, want %v", tc.vk, got, tc.want)
			}
		})
	}

	if len(hotkeyCommands) != 4 {
		t.Fatalf("hotkeyCommands has %d entries, want exactly 4", len(hotkeyCommands))
	}
	if _, ok := hotkeyCommands[0x00]; ok {
		t.Fatal("hotkeyCommands must not map unrelated vk codes")
	}
}
