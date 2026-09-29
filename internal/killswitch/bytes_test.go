package killswitch

import "testing"

func TestParseBytesUsesBinaryUnitsLikeActivityMonitor(t *testing.T) {
	cases := map[string]Bytes{
		"16GB":    16 * GiB,
		"16G":     16 * GiB,
		"16GiB":   16 * GiB,
		"16gb":    16 * GiB,
		"512MB":   512 * MiB,
		"1.5GB":   GiB + 512*MiB,
		"1TB":     1024 * GiB,
		"2048":    2048,
		" 8 GB ":  8 * GiB,
		"100KiB":  100 * KiB,
		"4096B":   4096,
		"0.5 TiB": 512 * GiB,
	}
	for input, want := range cases {
		got, err := ParseBytes(input)
		if err != nil {
			t.Errorf("ParseBytes(%q) returned error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseBytes(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseBytesRejectsMalformedAndNonPositiveSizes(t *testing.T) {
	for _, input := range []string{"", "GB", "sixteen", "16XB", "-1GB", "0", "0GB"} {
		if got, err := ParseBytes(input); err == nil {
			t.Errorf("ParseBytes(%q) = %d, want error", input, got)
		}
	}
}

func TestBytesFormatsWithLargestFittingUnit(t *testing.T) {
	cases := map[Bytes]string{
		246*GiB + 717*MiB: "246.70 GB",
		16 * GiB:          "16.00 GB",
		672 * MiB:         "672.00 MB",
		512:               "512 B",
	}
	for input, want := range cases {
		if got := input.String(); got != want {
			t.Errorf("Bytes(%d).String() = %q, want %q", uint64(input), got, want)
		}
	}
}
