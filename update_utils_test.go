package threadwave_test

import (
	"bytes"
	"testing"

	"github.com/Arnavsharma2/threadwave"
)

// TestEncodeStateVectorFromUpdate_MatchesDocSV: for a self-contained
// multi-client update, the state vector computed structurally from the
// update bytes equals the one computed from the reconstructed document.
func TestEncodeStateVectorFromUpdate_MatchesDocSV(t *testing.T) {
	a := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1})
	am := threadwave.NewMap(a, "m")
	wa := a.WriteTxn()
	am.Set(wa, "k1", "v1")
	am.Set(wa, "k1b", "v1b")
	wa.Commit()

	b := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 2})
	bm := threadwave.NewMap(b, "m")
	wb := b.WriteTxn()
	bm.Set(wb, "k2", "v2")
	wb.Commit()

	// Merge b into a so the update carries two clients.
	if err := threadwave.ApplyUpdate(a, threadwave.EncodeStateAsUpdate(b)); err != nil {
		t.Fatal(err)
	}

	update := threadwave.EncodeStateAsUpdate(a)
	svFromUpdate, err := threadwave.EncodeStateVectorFromUpdate(update)
	if err != nil {
		t.Fatal(err)
	}
	svFromDoc := threadwave.EncodeStateVector(a)
	if !bytes.Equal(svFromUpdate, svFromDoc) {
		t.Errorf("SV from update = %x, SV from doc = %x", svFromUpdate, svFromDoc)
	}
}

// TestEncodeStateVectorFromUpdate_OmitsNonZeroStart: a diff starts at a
// non-zero clock, so, like yjs, its client is omitted from the SV (a
// contiguous run from clock 0 is required). Over-reporting here would
// make a further diff withhold blocks the peer still needs.
func TestEncodeStateVectorFromUpdate_OmitsNonZeroStart(t *testing.T) {
	src := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1})
	m := threadwave.NewMap(src, "m")
	w1 := src.WriteTxn()
	m.Set(w1, "k1", "v1")
	w1.Commit()
	svAfter1 := threadwave.EncodeStateVector(src)
	w2 := src.WriteTxn()
	m.Set(w2, "k2", "v2")
	w2.Commit()

	// A diff past the first edit: client 1's blocks start at a non-zero clock.
	diff, err := threadwave.EncodeDiff(src, svAfter1)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := threadwave.EncodeStateVectorFromUpdate(diff)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sv, threadwave.EncodeStateVector(threadwave.NewDoc())) {
		t.Errorf("SV from a non-zero-start diff = %x, want empty (client omitted)", sv)
	}
}

// TestEncodeStateVectorFromUpdate_StopsAtSkip: for a client whose run is a
// 5-long prefix at clock 0 followed by a Skip block, the SV must report
// clock 5 (the contiguous prefix), matching yjs, not the post-Skip end.
func TestEncodeStateVectorFromUpdate_StopsAtSkip(t *testing.T) {
	// One client; a GC run [0,5) then a Skip [5,10).
	skipGap := []byte{0x01, 0x03, 0x01, 0x00, 0x00, 0x05, 0x0a, 0x05, 0x00, 0x05, 0x00}
	sv, err := threadwave.EncodeStateVectorFromUpdate(skipGap)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x01, 0x01, 0x05} // sv{client 1: clock 5}, matching yjs
	if !bytes.Equal(sv, want) {
		t.Errorf("SV stopping at Skip = %x, want %x", sv, want)
	}
}

// TestDiffUpdate_ExtractsMissing: a peer that already has part of a
// document receives, via DiffUpdate, exactly the blocks it is missing and
// converges.
func TestDiffUpdate_ExtractsMissing(t *testing.T) {
	src := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1})
	m := threadwave.NewMap(src, "m")

	// State after the first edit only.
	w1 := src.WriteTxn()
	m.Set(w1, "k1", "v1")
	w1.Commit()
	partial := threadwave.EncodeStateAsUpdate(src)

	// A second edit; the full update now carries both.
	w2 := src.WriteTxn()
	m.Set(w2, "k2", "v2")
	w2.Commit()
	full := threadwave.EncodeStateAsUpdate(src)

	// A remote that has only the first edit.
	remote := threadwave.NewDoc()
	if err := threadwave.ApplyUpdate(remote, partial); err != nil {
		t.Fatal(err)
	}
	rm := threadwave.NewMap(remote, "m")

	// The diff of the full update against the remote's SV carries only the
	// missing second edit; applying it converges the remote.
	diff, err := threadwave.DiffUpdate(full, threadwave.EncodeStateVector(remote))
	if err != nil {
		t.Fatal(err)
	}
	if err := threadwave.ApplyUpdate(remote, diff); err != nil {
		t.Fatal(err)
	}
	if rm.Get("k1") != "v1" || rm.Get("k2") != "v2" {
		t.Errorf("after diff apply: k1=%v k2=%v, want v1 v2", rm.Get("k1"), rm.Get("k2"))
	}
}

// TestDiffUpdate_AgainstEmptySVIsFull: diffing against an empty state
// vector yields the full state.
func TestDiffUpdate_AgainstEmptySVIsFull(t *testing.T) {
	src := threadwave.NewDoc()
	m := threadwave.NewMap(src, "m")
	w := src.WriteTxn()
	m.Set(w, "k", "v")
	w.Commit()

	diff, err := threadwave.DiffUpdate(threadwave.EncodeStateAsUpdate(src), threadwave.EncodeStateVector(threadwave.NewDoc()))
	if err != nil {
		t.Fatal(err)
	}
	target := threadwave.NewDoc()
	if err := threadwave.ApplyUpdate(target, diff); err != nil {
		t.Fatal(err)
	}
	if got := threadwave.NewMap(target, "m").Get("k"); got != "v" {
		t.Errorf("diff-against-empty Get(k) = %v, want v", got)
	}
}

// TestMergeUpdatesV2_Converges: two V2 updates from different clients
// merge into one V2 update that reconstructs the union.
func TestMergeUpdatesV2_Converges(t *testing.T) {
	a := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 1})
	am := threadwave.NewMap(a, "m")
	wa := a.WriteTxn()
	am.Set(wa, "k1", "v1")
	wa.Commit()

	b := threadwave.NewDocWithOptions(threadwave.Options{ClientID: 2})
	bm := threadwave.NewMap(b, "m")
	wb := b.WriteTxn()
	bm.Set(wb, "k2", "v2")
	wb.Commit()

	merged, err := threadwave.MergeUpdatesV2([][]byte{threadwave.EncodeStateAsUpdateV2(a), threadwave.EncodeStateAsUpdateV2(b)})
	if err != nil {
		t.Fatal(err)
	}

	target := threadwave.NewDoc()
	if err := threadwave.ApplyUpdateV2(target, merged); err != nil {
		t.Fatal(err)
	}
	tm := threadwave.NewMap(target, "m")
	if tm.Get("k1") != "v1" || tm.Get("k2") != "v2" {
		t.Errorf("merged V2 apply: k1=%v k2=%v, want v1 v2", tm.Get("k1"), tm.Get("k2"))
	}
}

// TestUpdateUtils_EmptyInputs: the empty-input contracts.
func TestUpdateUtils_EmptyInputs(t *testing.T) {
	if got, err := threadwave.MergeUpdatesV2(nil); err != nil || got != nil {
		t.Errorf("MergeUpdatesV2(nil) = %v, %v; want nil, nil", got, err)
	}
	// SV from the empty document's update is the empty SV (one zero byte:
	// a zero client count).
	sv, err := threadwave.EncodeStateVectorFromUpdate(threadwave.EncodeStateAsUpdate(threadwave.NewDoc()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sv, threadwave.EncodeStateVector(threadwave.NewDoc())) {
		t.Errorf("empty-update SV = %x, want %x", sv, threadwave.EncodeStateVector(threadwave.NewDoc()))
	}
}
