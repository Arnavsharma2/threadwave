package threadwave_test

import (
	"testing"

	"github.com/Arnavsharma2/threadwave"
)

// TestApplyUpdate_SkipStruct confirms an update carrying a yjs@14 Skip
// struct (ref 10) applies without error and leaves the document empty.
// Skip ranges mark holes in a peer's struct store; threadwave treats them as a
// forward-compatible no-op so v14 streams never crash the apply path.
func TestApplyUpdate_SkipStruct(t *testing.T) {
	d := threadwave.NewDoc()
	// numClients=1, blockCount=1, client=5, clock=0,
	// Skip block (info=10, len=3), empty delete set.
	buf := []byte{0x01, 0x01, 0x05, 0x00, 0x0a, 0x03, 0x00}
	if err := threadwave.ApplyUpdate(d, buf); err != nil {
		t.Fatalf("apply skip update: %v", err)
	}
}
