//go:build !darwin

package tunnel

import (
	"fmt"

	"github.com/YusufDrymz/unsni/internal/warp"
)

// Up is currently implemented only on macOS. Elsewhere, use `unsni warp` to
// generate a config and run it with WireGuard/wg-quick.
func Up(_ *warp.Account, _ func(string)) (func() error, error) {
	return nil, fmt.Errorf("embedded tunnel is macOS-only for now; use: unsni warp + wg-quick")
}
