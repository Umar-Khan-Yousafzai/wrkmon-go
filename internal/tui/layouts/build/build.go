package build

import (
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/layouts"
	_ "github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/layouts/single" // register
)

// DefaultLayout returns the default layout.
func DefaultLayout() layouts.Layout {
	return layouts.Get(layouts.Default())
}
