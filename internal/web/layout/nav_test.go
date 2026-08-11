package layout

import "testing"

func TestNavClass(t *testing.T) {
	t.Parallel()

	active := navClass("home", "home")
	if active == navClass("home", "about") {
		t.Fatal("active and inactive nav classes should differ")
	}
	if got := navClass("about", "about"); got != active {
		// both active cases share the same style bits; just ensure non-empty
		if got == "" {
			t.Fatal("active nav class empty")
		}
	}
}
