package threadwave_test

import (
	"testing"

	"github.com/Arnavsharma2/threadwave"
)

// collectMapEvents observes m and returns a slice the test appends to,
// one entry per fired event (each a snapshot of that event's Keys).
func collectMapEvents(d *threadwave.Doc, m *threadwave.Map) *[]map[string]threadwave.KeyChange {
	out := &[]map[string]threadwave.KeyChange{}
	m.Observe(func(e *threadwave.MapEvent) {
		snap := map[string]threadwave.KeyChange{}
		for k, v := range e.Keys {
			snap[k] = v
		}
		*out = append(*out, snap)
	})
	return out
}

func set(d *threadwave.Doc, m *threadwave.Map, key, val string) {
	txn := d.WriteTxn()
	m.Set(txn, key, val)
	txn.Commit()
}

func del(d *threadwave.Doc, m *threadwave.Map, key string) {
	txn := d.WriteTxn()
	m.Delete(txn, key)
	txn.Commit()
}

// TestMapObserve_Semantics checks threadwave's Map.Observe against the yjs
// YMapEvent semantics captured from yjs@13.6.31: add / update / delete
// actions, oldValue, and the empty-event firing for an add+delete in
// one transaction.
func TestMapObserve_Semantics(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		d := threadwave.NewDoc()
		m := threadwave.NewMap(d, "m")
		ev := collectMapEvents(d, m)
		set(d, m, "a", "1")
		assertEvents(t, *ev, []map[string]threadwave.KeyChange{
			{"a": {Action: "add"}},
		})
	})

	t.Run("update across two txns", func(t *testing.T) {
		d := threadwave.NewDoc()
		m := threadwave.NewMap(d, "m")
		ev := collectMapEvents(d, m)
		set(d, m, "a", "1")
		set(d, m, "a", "2")
		assertEvents(t, *ev, []map[string]threadwave.KeyChange{
			{"a": {Action: "add"}},
			{"a": {Action: "update", OldValue: "1"}},
		})
	})

	t.Run("delete across two txns", func(t *testing.T) {
		d := threadwave.NewDoc()
		m := threadwave.NewMap(d, "m")
		ev := collectMapEvents(d, m)
		set(d, m, "a", "1")
		del(d, m, "a")
		assertEvents(t, *ev, []map[string]threadwave.KeyChange{
			{"a": {Action: "add"}},
			{"a": {Action: "delete", OldValue: "1"}},
		})
	})

	t.Run("add then delete in one txn fires empty event", func(t *testing.T) {
		d := threadwave.NewDoc()
		m := threadwave.NewMap(d, "m")
		ev := collectMapEvents(d, m)
		txn := d.WriteTxn()
		m.Set(txn, "a", "1")
		m.Delete(txn, "a")
		txn.Commit()
		assertEvents(t, *ev, []map[string]threadwave.KeyChange{{}})
	})

	t.Run("update in one txn over a prior value", func(t *testing.T) {
		d := threadwave.NewDoc()
		m := threadwave.NewMap(d, "m")
		set(d, m, "a", "1")
		ev := collectMapEvents(d, m)
		txn := d.WriteTxn()
		m.Set(txn, "a", "2")
		txn.Commit()
		assertEvents(t, *ev, []map[string]threadwave.KeyChange{
			{"a": {Action: "update", OldValue: "1"}},
		})
	})

	t.Run("multi-key one txn", func(t *testing.T) {
		d := threadwave.NewDoc()
		m := threadwave.NewMap(d, "m")
		ev := collectMapEvents(d, m)
		txn := d.WriteTxn()
		m.Set(txn, "x", "X")
		m.Set(txn, "y", "Y")
		txn.Commit()
		assertEvents(t, *ev, []map[string]threadwave.KeyChange{
			{"x": {Action: "add"}, "y": {Action: "add"}},
		})
	})
}

// TestMapObserve_RemoteUpdate confirms observers fire when a remote
// update is applied (not just on local edits).
func TestMapObserve_RemoteUpdate(t *testing.T) {
	src := threadwave.NewDoc()
	sm := threadwave.NewMap(src, "m")
	set(src, sm, "k", "v")

	dst := threadwave.NewDoc()
	dm := threadwave.NewMap(dst, "m")
	ev := collectMapEvents(dst, dm)
	if err := threadwave.ApplyUpdate(dst, threadwave.EncodeStateAsUpdate(src)); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, *ev, []map[string]threadwave.KeyChange{
		{"k": {Action: "add"}},
	})
}

// TestMapObserve_Unsubscribe confirms the returned function detaches.
func TestMapObserve_Unsubscribe(t *testing.T) {
	d := threadwave.NewDoc()
	m := threadwave.NewMap(d, "m")
	fired := 0
	unsub := m.Observe(func(*threadwave.MapEvent) { fired++ })
	set(d, m, "a", "1")
	unsub()
	set(d, m, "b", "2")
	if fired != 1 {
		t.Errorf("fired %d times, want 1 (second edit after unsubscribe)", fired)
	}
}

func assertEvents(t *testing.T, got, want []map[string]threadwave.KeyChange) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Errorf("event %d: got %d keys, want %d: %+v vs %+v", i, len(got[i]), len(want[i]), got[i], want[i])
			continue
		}
		for k, wc := range want[i] {
			gc, ok := got[i][k]
			if !ok {
				t.Errorf("event %d: missing key %q", i, k)
				continue
			}
			if gc.Action != wc.Action {
				t.Errorf("event %d key %q: action %q, want %q", i, k, gc.Action, wc.Action)
			}
			if gc.OldValue != wc.OldValue {
				t.Errorf("event %d key %q: oldValue %#v, want %#v", i, k, gc.OldValue, wc.OldValue)
			}
		}
	}
}
