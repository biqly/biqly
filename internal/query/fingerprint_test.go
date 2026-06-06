package query

import "testing"

func mustComputeFingerprint(t *testing.T, in FingerprintInputs) string {
	t.Helper()
	fingerprint, err := ComputeFingerprint(in)
	if err != nil {
		t.Fatalf("ComputeFingerprint(%+v) error = %v", in, err)
	}
	return fingerprint
}

func TestComputeFingerprintIsStable(t *testing.T) {
	in := FingerprintInputs{
		LogicalQuery: &LogicalQuery{
			Version:      CurrentLogicalQueryVersion,
			DatasourceID: "ds1",
			ModelID:      "orders",
			Select: []SelectItem{
				{Type: SelectTypeDimension, Name: "country"},
				{Type: SelectTypeMetric, Name: "order_count"},
			},
			Filters: []Filter{
				{Field: "created_at", Operator: OpGte, Value: "2026-01-01"},
			},
			GroupBy: []GroupBy{{Field: "country"}},
			Limit:   100,
		},
		DatasourceID:   "ds1",
		ContextVersion: "3",
	}
	a := mustComputeFingerprint(t, in)
	b := mustComputeFingerprint(t, in)
	if a == "" {
		t.Fatal("fingerprint must not be empty")
	}
	if a != b {
		t.Errorf("fingerprint is non-deterministic: %q vs %q", a, b)
	}
}

func TestComputeFingerprintReturnsMarshalError(t *testing.T) {
	_, err := ComputeFingerprint(FingerprintInputs{
		LogicalQuery: &LogicalQuery{
			ModelID: "orders",
			Filters: []Filter{
				{Field: "country", Operator: OpEq, Value: func() {}},
			},
		},
		DatasourceID: "ds1",
	})
	if err == nil {
		t.Fatal("ComputeFingerprint() error = nil, want marshal error")
	}
}

func TestComputeFingerprintIgnoresVersionField(t *testing.T) {
	base := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "orders",
		Select:       []SelectItem{{Type: SelectTypeMetric, Name: "order_count"}},
		Limit:        10,
	}
	a := mustComputeFingerprint(t, FingerprintInputs{LogicalQuery: &base, DatasourceID: "ds1"})

	base.Version = "v2-experimental"
	b := mustComputeFingerprint(t, FingerprintInputs{LogicalQuery: &base, DatasourceID: "ds1"})

	if a != b {
		t.Errorf("fingerprint must not depend on LogicalQuery.Version: %q vs %q", a, b)
	}
}

func TestComputeFingerprintCanonicalizesFilterOrder(t *testing.T) {
	mk := func(filters []Filter) FingerprintInputs {
		return FingerprintInputs{
			LogicalQuery: &LogicalQuery{
				DatasourceID: "ds1",
				ModelID:      "orders",
				Select:       []SelectItem{{Type: SelectTypeMetric, Name: "order_count"}},
				Filters:      filters,
			},
			DatasourceID: "ds1",
		}
	}
	a := mustComputeFingerprint(t, mk([]Filter{
		{Field: "country", Operator: OpEq, Value: "TR"},
		{Field: "created_at", Operator: OpGte, Value: "2026-01-01"},
	}))
	b := mustComputeFingerprint(t, mk([]Filter{
		{Field: "created_at", Operator: OpGte, Value: "2026-01-01"},
		{Field: "country", Operator: OpEq, Value: "TR"},
	}))
	if a != b {
		t.Errorf("filter order should not change fingerprint: %q vs %q", a, b)
	}
}

func TestComputeFingerprintDistinguishesContextVersion(t *testing.T) {
	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "orders",
		Select:       []SelectItem{{Type: SelectTypeMetric, Name: "order_count"}},
	}
	a := mustComputeFingerprint(t, FingerprintInputs{LogicalQuery: &lq, DatasourceID: "ds1", ContextVersion: "1"})
	b := mustComputeFingerprint(t, FingerprintInputs{LogicalQuery: &lq, DatasourceID: "ds1", ContextVersion: "2"})
	if a == b {
		t.Error("different context versions must produce different fingerprints")
	}
}

func TestComputeFingerprintDistinguishesPermissionScope(t *testing.T) {
	lq := LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "orders",
		Select:       []SelectItem{{Type: SelectTypeMetric, Name: "order_count"}},
	}
	a := mustComputeFingerprint(t, FingerprintInputs{LogicalQuery: &lq, DatasourceID: "ds1", PermissionScope: "user-a"})
	b := mustComputeFingerprint(t, FingerprintInputs{LogicalQuery: &lq, DatasourceID: "ds1", PermissionScope: "user-b"})
	if a == b {
		t.Error("different permission scopes must produce different fingerprints")
	}
}
