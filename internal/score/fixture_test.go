package score

import "testing"

func TestFixtureDeterministic(t *testing.T) {
	f := Fixture{}
	a, err := f.Score(nil, "cursor")
	if err != nil || a != 82 {
		t.Fatalf("cursor=%v err=%v", a, err)
	}
	b, _ := f.Score(nil, "cursor")
	if a != b {
		t.Fatal("not deterministic")
	}
	x, _ := f.Score(nil, "unknown_type_xyz")
	y, _ := f.Score(nil, "unknown_type_xyz")
	if x != y || x < 40 || x > 90 {
		t.Fatalf("hash score=%v", x)
	}
}

func TestResolveProviderName(t *testing.T) {
	if ResolveProviderName("", false) != "fixture" {
		t.Fatal("default fixture")
	}
	if ResolveProviderName("auto", true) != "guardloop" {
		t.Fatal("auto+enabled")
	}
	if ResolveProviderName("noop", true) != "noop" {
		t.Fatal("explicit noop")
	}
}
