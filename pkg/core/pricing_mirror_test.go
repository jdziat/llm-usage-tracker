package core

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The desktop app hand-mirrors this package's price, fast-multiplier and
// family tables in desktop/frontend/src/lib/palette.ts, and presents its
// verdict to the user as the authoritative provenance line. Nothing else fails
// when the two drift, so a Go-side price change that misses the TypeScript
// edit would silently label a priced model unknown, or attach the wrong
// estimate label. These tests read that file and compare.

const palettePath = "../../desktop/frontend/src/lib/palette.ts"

func readPalette(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(palettePath)
	if err != nil {
		t.Fatalf("read %s: %v", palettePath, err)
	}
	return string(b)
}

// section returns the text between a declaration and the line that closes it.
func section(t *testing.T, src, start, end string) string {
	t.Helper()
	i := strings.Index(src, start)
	if i < 0 {
		t.Fatalf("%s: %q not found; the mirror was renamed or removed", palettePath, start)
	}
	rest := src[i:]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("%s: no %q after %q", palettePath, end, start)
	}
	return rest[:j]
}

func TestDesktopMirrorsPriceTable(t *testing.T) {
	body := section(t, readPalette(t), "export const MODEL_PRICES", "\n};")
	row := regexp.MustCompile(`'([^']+)':\s*\[([\d.]+),\s*([\d.]+)\]`)
	seen := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(body, -1) {
		key, in, out := m[1], mustFloat(t, m[2]), mustFloat(t, m[3])
		want, ok := builtinPrices[key]
		if !ok {
			t.Errorf("palette.ts prices %q, which is not in builtinPrices", key)
			continue
		}
		if in != want.input || out != want.output {
			t.Errorf("palette.ts %q = %g/%g, want %g/%g", key, in, out, want.input, want.output)
		}
		seen[key] = true
	}
	for key := range builtinPrices {
		if !seen[key] {
			t.Errorf("builtinPrices has %q; palette.ts MODEL_PRICES does not", key)
		}
	}
}

func TestDesktopMirrorsFastMultipliers(t *testing.T) {
	body := section(t, readPalette(t), "export const FAST_MULTIPLIER", "\n};")
	row := regexp.MustCompile(`'([^']+)':\s*([\d.]+)`)
	seen := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(body, -1) {
		key, mult := m[1], mustFloat(t, m[2])
		want, ok := fastMultipliers[key]
		if !ok {
			t.Errorf("palette.ts gives %q a fast multiplier; fastMultipliers does not", key)
			continue
		}
		if mult != want {
			t.Errorf("palette.ts %q fast multiplier = %g, want %g", key, mult, want)
		}
		seen[key] = true
	}
	for key := range fastMultipliers {
		if !seen[key] {
			t.Errorf("fastMultipliers has %q; palette.ts FAST_MULTIPLIER does not", key)
		}
	}
}

func TestDesktopMirrorsFamilyDefaults(t *testing.T) {
	body := section(t, readPalette(t), "const FAMILY_DEFAULTS", "\n];")
	row := regexp.MustCompile(`tokens:\s*\[([^\]]*)\],\s*key:\s*'([^']+)'`)
	got := [][2]string{}
	for _, m := range row.FindAllStringSubmatch(body, -1) {
		tokens := strings.NewReplacer("'", "", " ", "").Replace(m[1])
		got = append(got, [2]string{tokens, m[2]})
	}
	if len(got) != len(familyDefaults) {
		t.Fatalf("palette.ts has %d family defaults, want %d", len(got), len(familyDefaults))
	}
	// Order matters in both: the first matching entry wins.
	for i, f := range familyDefaults {
		wantTokens := strings.Join(f.tokens, ",")
		if got[i][0] != wantTokens || got[i][1] != f.key {
			t.Errorf("palette.ts family default %d = %v, want [%s %s]", i, got[i], wantTokens, f.key)
		}
	}
}

func mustFloat(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return f
}
