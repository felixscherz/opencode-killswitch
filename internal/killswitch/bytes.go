package killswitch

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Bytes is an amount of memory. Units are binary (1 GB = 1024^3 bytes),
// matching how Activity Monitor reports memory.
type Bytes uint64

const (
	KiB Bytes = 1 << (10 * (iota + 1))
	MiB
	GiB
	TiB
)

var unitsBySuffix = map[string]Bytes{
	"":    1,
	"b":   1,
	"k":   KiB,
	"kb":  KiB,
	"kib": KiB,
	"m":   MiB,
	"mb":  MiB,
	"mib": MiB,
	"g":   GiB,
	"gb":  GiB,
	"gib": GiB,
	"t":   TiB,
	"tb":  TiB,
	"tib": TiB,
}

// ParseBytes parses a positive size such as "16GB", "512 MiB" or "1.5g".
func ParseBytes(s string) (Bytes, error) {
	trimmed := strings.TrimSpace(s)
	split := strings.IndexFunc(trimmed, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	if split == -1 {
		split = len(trimmed)
	}
	number, suffix := trimmed[:split], strings.ToLower(strings.TrimSpace(trimmed[split:]))

	unit, ok := unitsBySuffix[suffix]
	if !ok {
		return 0, fmt.Errorf("invalid size %q: unknown unit %q (use B, KB, MB, GB or TB)", s, suffix)
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: expected a number followed by a unit, e.g. 16GB", s)
	}
	size := value * float64(unit)
	if size < 1 || size > math.MaxUint64 {
		return 0, fmt.Errorf("invalid size %q: must be positive", s)
	}
	return Bytes(size), nil
}

func (b Bytes) String() string {
	for _, u := range []struct {
		size Bytes
		name string
	}{{TiB, "TB"}, {GiB, "GB"}, {MiB, "MB"}, {KiB, "KB"}} {
		if b >= u.size {
			return fmt.Sprintf("%.2f %s", float64(b)/float64(u.size), u.name)
		}
	}
	return fmt.Sprintf("%d B", uint64(b))
}
