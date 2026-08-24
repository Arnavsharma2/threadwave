package threadwave_test

import (
	"reflect"
	"testing"

	"github.com/Arnavsharma2/threadwave"
)

// TestPublicAPI_Smoke proves an external caller can do every
// primary operation using only the threadwave package — no internal/*
// imports. This is the contract a gomobile-bind user (and any
// adopter writing against the public API) would follow.
func TestPublicAPI_Smoke(t *testing.T) {
	d := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1000})

	m := threadwave.NewMap(d, "settings")
	arr := threadwave.NewArray(d, "items")
	tx := threadwave.NewText(d, "body")

	w := d.WriteTxn()
	m.Set(w, "color", "blue")
	arr.Push(w, "alpha", "beta")
	_ = tx.InsertWithAttributes(w, 0, "hello", threadwave.Attrs{"bold": true})
	w.Commit()

	// Reads — Map.Get, Array.ToSlice, Text.ToDelta.
	if got := m.Get("color"); got != "blue" {
		t.Errorf("Map color = %v, want blue", got)
	}
	if got := arr.Len(); got != 2 {
		t.Errorf("Array Len = %d, want 2", got)
	}
	delta := tx.ToDelta()
	want := []threadwave.DeltaOp{{Insert: "hello", Attributes: threadwave.Attrs{"bold": true}}}
	if !reflect.DeepEqual(delta, want) {
		t.Errorf("ToDelta = %+v, want %+v", delta, want)
	}
}

func TestPublicAPI_WireRoundTrip(t *testing.T) {
	src := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1100})
	m := threadwave.NewMap(src, "k")
	w := src.WriteTxn()
	m.Set(w, "a", "1")
	w.Commit()

	bytes := threadwave.EncodeStateAsUpdate(src)

	target := threadwave.NewDoc()
	if err := threadwave.ApplyUpdate(target, bytes); err != nil {
		t.Fatal(err)
	}
	tm := threadwave.NewMap(target, "k")
	if got := tm.Get("a"); got != "1" {
		t.Errorf("target a = %v, want 1", got)
	}
}

func TestPublicAPI_StateVectorAndDiff(t *testing.T) {
	src := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1200})
	arr := threadwave.NewArray(src, "items")
	w := src.WriteTxn()
	arr.Push(w, "first")
	w.Commit()

	sv := threadwave.EncodeStateVector(src)
	if len(sv) == 0 {
		t.Error("EncodeStateVector returned empty bytes")
	}

	// Encode a diff against the same SV — should be effectively empty
	// (one varuint(0) for the client list + one varuint(0) for the DS).
	diff, err := threadwave.EncodeDiff(src, sv)
	if err != nil {
		t.Fatal(err)
	}
	// Apply to a target — should be a no-op.
	target := threadwave.NewDoc()
	if err := threadwave.ApplyUpdate(target, diff); err != nil {
		t.Fatal(err)
	}
	tArr := threadwave.NewArray(target, "items")
	if tArr.Len() != 0 {
		t.Errorf("target Array Len after empty-diff apply = %d, want 0", tArr.Len())
	}

	// Encode a diff against the empty SV — should carry the array item.
	diffEmpty, err := threadwave.EncodeDiff(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	target2 := threadwave.NewDoc()
	if err := threadwave.ApplyUpdate(target2, diffEmpty); err != nil {
		t.Fatal(err)
	}
	tArr2 := threadwave.NewArray(target2, "items")
	if tArr2.Len() != 1 {
		t.Errorf("target2 Array Len = %d, want 1", tArr2.Len())
	}
}

func TestPublicAPI_PendingHelpers(t *testing.T) {
	d := threadwave.NewDoc()
	if threadwave.HasPending(d) {
		t.Error("fresh Doc reports HasPending")
	}
	if missing := threadwave.MissingSV(d); len(missing) == 0 || missing[0] != 0x00 {
		// Encoded empty SV is varuint(0) = single 0x00 byte.
		t.Errorf("fresh Doc MissingSV = %x, want [00]", missing)
	}
}

func TestPublicAPI_MergeUpdates(t *testing.T) {
	a := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1300})
	am := threadwave.NewMap(a, "m")
	w := a.WriteTxn()
	am.Set(w, "k", "v")
	w.Commit()

	b := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1301})
	bm := threadwave.NewMap(b, "m")
	w = b.WriteTxn()
	bm.Set(w, "k2", "v2")
	w.Commit()

	merged, err := threadwave.MergeUpdates([][]byte{
		threadwave.EncodeStateAsUpdate(a),
		threadwave.EncodeStateAsUpdate(b),
	})
	if err != nil {
		t.Fatal(err)
	}

	target := threadwave.NewDoc()
	if err := threadwave.ApplyUpdate(target, merged); err != nil {
		t.Fatal(err)
	}
	tm := threadwave.NewMap(target, "m")
	if tm.Get("k") != "v" || tm.Get("k2") != "v2" {
		t.Errorf("merged target missing keys: k=%v k2=%v", tm.Get("k"), tm.Get("k2"))
	}
}

func TestPublicAPI_Awareness(t *testing.T) {
	d := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1400})
	a := threadwave.NewAwareness(d.ClientID())
	a.SetLocalState([]byte(`{"name":"Alice"}`))

	state, ok := a.LocalState()
	if !ok || string(state) != `{"name":"Alice"}` {
		t.Errorf("LocalState = (%q, %v), want Alice", state, ok)
	}

	// DefaultAwarenessTimeout is a public constant.
	_ = threadwave.DefaultAwarenessTimeout
}

func TestPublicAPI_NestedTypesAndXml(t *testing.T) {
	d := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1500})

	// Nested Map-in-Map.
	root := threadwave.NewMap(d, "root")
	w := d.WriteTxn()
	inner := root.SetMap(w, "inner")
	inner.Set(w, "k", "v")
	w.Commit()

	got := root.Get("inner")
	innerBack, ok := got.(*threadwave.Map)
	if !ok {
		t.Fatalf("root.Get(inner) = %T, want *threadwave.Map", got)
	}
	if v := innerBack.Get("k"); v != "v" {
		t.Errorf("inner.Get(k) = %v, want v", v)
	}

	// XML.
	frag := threadwave.NewXmlFragment(d, "page")
	w = d.WriteTxn()
	p := frag.InsertXmlElement(w, 0, "p")
	p.SetAttribute(w, "class", "lede")
	pText := p.InsertXmlText(w, 0)
	_ = pText.Insert(w, 0, "hello")
	w.Commit()

	if got, want := frag.ToString(), `<p class="lede">hello</p>`; got != want {
		t.Errorf("XmlFragment.ToString = %q, want %q", got, want)
	}
}

// TestPublicAPI_Version exposes Version as a public constant.
func TestPublicAPI_Version(t *testing.T) {
	if threadwave.Version == "" {
		t.Error("Version is empty")
	}
}
