package adapters

import "testing"

func TestNineAdapterTypes(t *testing.T) {
	if len(AllTypes) != 9 {
		t.Fatalf("expected 9 types, got %d", len(AllTypes))
	}
	for _, typ := range AllTypes {
		if _, ok := Manifests[typ]; !ok {
			t.Fatalf("missing manifest for %s", typ)
		}
	}
}
