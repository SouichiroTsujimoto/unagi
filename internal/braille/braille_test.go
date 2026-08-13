package braille

import "testing"

func TestNoiseGridIsStablePerSeed(t *testing.T) {
	first := NoiseGrid("hello-unagi", 5, 5)
	second := NoiseGrid("hello-unagi", 5, 5)
	other := NoiseGrid("kea-dhcp-lease-metrics", 5, 5)

	if first.Pack01() != second.Pack01() {
		t.Fatal("same seed produced different patterns")
	}
	if first.Pack01() == other.Pack01() {
		t.Fatal("different seeds produced the same pattern")
	}
	if len(first.On) != 25 {
		t.Fatalf("pattern size=%d", len(first.On))
	}
}
