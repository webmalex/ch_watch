package queue

import "testing"

func TestOrderedSetPreservesFirstSeenOrder(t *testing.T) {
	t.Parallel()

	set := NewOrderedSet()
	if !set.Add("b.sql") {
		t.Fatal("expected first add to succeed")
	}
	set.Add("a.sql")
	set.Add("b.sql")

	first, ok := set.PopFront()
	if !ok || first != "b.sql" {
		t.Fatalf("unexpected first item: %q", first)
	}
	second, ok := set.PopFront()
	if !ok || second != "a.sql" {
		t.Fatalf("unexpected second item: %q", second)
	}
	if _, ok := set.PopFront(); ok {
		t.Fatal("expected set to be empty")
	}
}
