package query

import (
	"testing"
)

func TestBorrowScanSlice_Small(t *testing.T) {
	slice, pooled := borrowScanSlice(5)
	if len(slice) != 5 {
		t.Fatalf("len(slice) = %d, want 5", len(slice))
	}
	if cap(slice) < 5 {
		t.Fatalf("cap(slice) = %d, want >= 5", cap(slice))
	}
	// pooled may be nil (first call) or non-nil (reuse) — either is fine
	_ = pooled
}

func TestBorrowScanSlice_ReturnAndReuse(t *testing.T) {
	// Borrow, return to pool, borrow again — should get same backing array
	slice, pooled := borrowScanSlice(16)
	if pooled == nil {
		t.Skip("first call got a fresh allocation, nothing to recycle")
	}
	_ = slice

	// Put back manually (defer in scanRows normally does this)
	for i := range *pooled {
		(*pooled)[i] = nil
	}
	scanSlicePool.Put(pooled)

	// Borrow again — should reuse
	slice2, pooled2 := borrowScanSlice(16)
	if pooled2 == nil {
		t.Fatal("expected pooled pointer after returning to pool")
	}
	_ = slice2
	_ = pooled2
}

func TestBorrowScanSlice_NeedLarger(t *testing.T) {
	// Put a small slice back
	small := make([]any, 5)
	scanSlicePool.Put(&small)

	// Request larger — should not reuse the small one
	slice, pooled := borrowScanSlice(100)
	if len(slice) != 100 {
		t.Fatalf("len(slice) = %d, want 100", len(slice))
	}
	if pooled != nil {
		t.Fatal("expected nil pooled pointer because small slice was too small")
	}
}
