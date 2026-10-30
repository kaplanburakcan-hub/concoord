package schedule

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Baseline anlık görüntüsünden PV: iki yaprak, ağırlık 60/40, sırayla bitiyor.
func TestPlannedPhysicalFromSnapshot(t *testing.T) {
	s1, f1 := "2026-01-01", "2026-01-31"
	s2, f2 := "2026-02-01", "2026-02-28"
	snap, err := json.Marshal([]baselineSnapshotItem{
		{ID: uuid.New(), WbsCode: "1", Weight: 60, BaselineStart: &s1, BaselineFinish: &f1},
		{ID: uuid.New(), WbsCode: "2", Weight: 40, BaselineStart: &s2, BaselineFinish: &f2},
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		at   time.Time
		want float64
	}{
		{time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), 0},
		{time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), 60},
		{time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC), 100},
	}
	for _, c := range cases {
		got, err := PlannedPhysicalFromSnapshot(snap, c.at)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(got-c.want) > 0.01 {
			t.Errorf("at %s: got %.2f want %.2f", c.at.Format("2006-01-02"), got, c.want)
		}
	}
}

func TestPlannedPhysicalFromSnapshotBadJSON(t *testing.T) {
	if _, err := PlannedPhysicalFromSnapshot([]byte("{"), time.Now()); err == nil {
		t.Fatal("bozuk JSON hata dönmeli")
	}
}
