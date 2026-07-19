package spec

import (
	"strings"
	"testing"
)

func TestCanonicalSpecBytes_KeyOrderStable(t *testing.T) {
	a := []byte(`{"b":1,"a":2,"c":{"y":1,"x":2}}`)
	b := []byte(`{"a":2,"c":{"x":2,"y":1},"b":1}`)
	ba, err := CanonicalSpecBytes(a)
	if err != nil {
		t.Fatalf("CanonicalSpecBytes a: %v", err)
	}
	bb, err := CanonicalSpecBytes(b)
	if err != nil {
		t.Fatalf("CanonicalSpecBytes b: %v", err)
	}
	if string(ba) != string(bb) {
		t.Fatalf("canonical outputs differ:\n a=%s\n b=%s", ba, bb)
	}
	if !strings.HasPrefix(string(ba), `{"a":`) {
		t.Fatalf("expected keys sorted, got %s", ba)
	}
}

func TestCanonicalSpecBytes_YAMLAndJSONMatch(t *testing.T) {
	y := []byte("a: 1\nb:\n  - 3\n  - 2\n")
	j := []byte(`{"b":[3,2],"a":1}`)
	by, err := CanonicalSpecBytes(y)
	if err != nil {
		t.Fatalf("yaml: %v", err)
	}
	bj, err := CanonicalSpecBytes(j)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if string(by) != string(bj) {
		t.Fatalf("yaml/json canonical mismatch:\n yaml=%s\n json=%s", by, bj)
	}
}

func TestCanonicalSpecBytes_ArrayOrderPreserved(t *testing.T) {
	// arrays are semantically ordered — do NOT sort
	a := []byte(`{"steps":[{"name":"a"},{"name":"b"}]}`)
	b := []byte(`{"steps":[{"name":"b"},{"name":"a"}]}`)
	ba, _ := CanonicalSpecBytes(a)
	bb, _ := CanonicalSpecBytes(b)
	if string(ba) == string(bb) {
		t.Fatal("array reordering should change canonical output")
	}
}

func TestLibraryReleaseHash_StableAcrossAliasOrder(t *testing.T) {
	spec := []byte(`{"schemaVersion":1,"metadata":{"slug":"deploy"}}`)
	canon, _ := CanonicalSpecBytes(spec)

	h1 := LibraryReleaseHash(LibraryReleaseHashInput{
		Slug: "lib", Name: "Lib", Description: "d",
		Aliases: []string{"b", "a"},
		Specs:   []SpecHashEntry{{Slug: "deploy", Bytes: canon}},
	})
	h2 := LibraryReleaseHash(LibraryReleaseHashInput{
		Slug: "lib", Name: "Lib", Description: "d",
		Aliases: []string{"a", "b"},
		Specs:   []SpecHashEntry{{Slug: "deploy", Bytes: canon}},
	})
	if h1 != h2 {
		t.Errorf("alias order changed hash: %s vs %s", h1, h2)
	}
}

func TestLibraryReleaseHash_StableAcrossSpecOrder(t *testing.T) {
	sA, _ := CanonicalSpecBytes([]byte(`{"metadata":{"slug":"a"}}`))
	sB, _ := CanonicalSpecBytes([]byte(`{"metadata":{"slug":"b"}}`))

	h1 := LibraryReleaseHash(LibraryReleaseHashInput{
		Slug: "lib", Name: "Lib", Aliases: nil,
		Specs: []SpecHashEntry{{Slug: "a", Bytes: sA}, {Slug: "b", Bytes: sB}},
	})
	h2 := LibraryReleaseHash(LibraryReleaseHashInput{
		Slug: "lib", Name: "Lib", Aliases: nil,
		Specs: []SpecHashEntry{{Slug: "b", Bytes: sB}, {Slug: "a", Bytes: sA}},
	})
	if h1 != h2 {
		t.Errorf("spec order changed hash: %s vs %s", h1, h2)
	}
}

func TestLibraryReleaseHash_ChangesWithContent(t *testing.T) {
	sA, _ := CanonicalSpecBytes([]byte(`{"metadata":{"slug":"a"},"v":1}`))
	sAModified, _ := CanonicalSpecBytes([]byte(`{"metadata":{"slug":"a"},"v":2}`))

	h1 := LibraryReleaseHash(LibraryReleaseHashInput{
		Slug: "lib", Name: "Lib",
		Specs: []SpecHashEntry{{Slug: "a", Bytes: sA}},
	})
	h2 := LibraryReleaseHash(LibraryReleaseHashInput{
		Slug: "lib", Name: "Lib",
		Specs: []SpecHashEntry{{Slug: "a", Bytes: sAModified}},
	})
	if h1 == h2 {
		t.Errorf("content change did not affect hash: %s", h1)
	}
}

func TestLibraryReleaseHash_HasVersionPrefix(t *testing.T) {
	h := LibraryReleaseHash(LibraryReleaseHashInput{Slug: "lib", Name: "Lib"})
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("expected sha256: prefix, got %s", h)
	}
}
