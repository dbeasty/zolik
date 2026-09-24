package db

import "testing"

func TestDynamicNamespaceNames(t *testing.T) {
	cases := []struct {
		name    string
		dynamic bool
	}{
		{UserNS("65f0c0ffee"), true},
		{UserReadOnlyNS("65f0c0ffee"), true},
		{MatchNS("65f0dead"), true},
		{NodeOutboxNS("node-1"), true},
		{NSUsers, false},
		{"", false},
		{"u/", false},
		// A name with a separator in the part that is data would let a caller
		// reach outside its own subtree, which is the whole point of naming
		// namespaces per user in the first place.
		{"u/65f0/../other", false},
		{"m/65f0/m/1", false},
		{"node/n1", false},
		{"node//outbox", false},
	}
	for _, c := range cases {
		if got := IsDynamicNamespace(c.name); got != c.dynamic {
			t.Errorf("IsDynamicNamespace(%q) = %v, want %v", c.name, got, c.dynamic)
		}
	}
}

func TestNamespaceOwnership(t *testing.T) {
	if user, ok := UserOfNamespace(UserNS("abc")); !ok || user != "abc" {
		t.Fatalf("user of %q = %q, %v", UserNS("abc"), user, ok)
	}
	if user, ok := UserOfNamespace(UserReadOnlyNS("abc")); !ok || user != "abc" {
		t.Fatalf("user of %q = %q, %v", UserReadOnlyNS("abc"), user, ok)
	}
	if _, ok := UserOfNamespace(MatchNS("abc")); ok {
		t.Fatal("a match namespace must not read as a user's")
	}
	if m, ok := MatchOfNamespace(MatchNS("abc")); !ok || m != "abc" {
		t.Fatalf("match of %q = %q, %v", MatchNS("abc"), m, ok)
	}
	if n, ok := NodeOfOutbox(NodeOutboxNS("n1")); !ok || n != "n1" {
		t.Fatalf("node of %q = %q, %v", NodeOutboxNS("n1"), n, ok)
	}
}

func TestNamespacesOpenOnDemandAndStayOpen(t *testing.T) {
	k := openTestKDB(t)
	ns := UserNS("65f0c0ffee")
	if err := k.Put(ns, "prefs", []byte(`{"lang":"cs"}`)); err != nil {
		t.Fatalf("put into a namespace nobody declared: %v", err)
	}
	doc, err := k.Get(ns, "prefs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(doc) == 0 {
		t.Fatal("empty document back")
	}
	// The second use must find the namespace already open rather than opening
	// a second runtime over it: two runtimes over one namespace would be two
	// uncoordinated writers.
	first, err := k.Namespace(ns)
	if err != nil {
		t.Fatalf("namespace: %v", err)
	}
	second, err := k.Namespace(ns)
	if err != nil {
		t.Fatalf("namespace again: %v", err)
	}
	if first != second {
		t.Fatal("a namespace was opened twice")
	}
	if _, err := k.Namespace("not_a_declared_namespace"); err == nil {
		t.Fatal("an undeclared, non-dynamic name must not open")
	}
}
