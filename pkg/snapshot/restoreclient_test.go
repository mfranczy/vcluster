package snapshot

import (
	"testing"
)

func TestSnapshotRestoreBumpRevision(t *testing.T) {
	tests := []struct {
		name             string
		latestRevision   int64
		snapshotRevision int64
		bumpRevision     int64
		expected         uint64
	}{
		{
			name:             "latest greater than snapshot",
			latestRevision:   1500,
			snapshotRevision: 1000,
			bumpRevision:     1000,
			expected:         1500,
		},
		{
			name:             "snapshot greater than latest",
			latestRevision:   800,
			snapshotRevision: 1000,
			bumpRevision:     1000,
			expected:         1000,
		},
		{
			name:             "latest equal to snapshot",
			latestRevision:   1000,
			snapshotRevision: 1000,
			bumpRevision:     50,
			expected:         50,
		},
		{
			name:             "negative values",
			latestRevision:   -500,
			snapshotRevision: -1000,
			bumpRevision:     100,
			expected:         600,
		},
		{
			name:             "all zeros",
			latestRevision:   0,
			snapshotRevision: 0,
			bumpRevision:     1000,
			expected:         1000,
		},
		{
			name:             "snapshot and bump zero",
			latestRevision:   500,
			snapshotRevision: 0,
			bumpRevision:     1000,
			expected:         1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := snapshotRestoreBumpRevision(tt.latestRevision, tt.snapshotRevision, tt.bumpRevision)
			if result != tt.expected {
				t.Errorf("snapshotRestoreBumpRevision(%d, %d, %d) = %d; want %d",
					tt.latestRevision, tt.snapshotRevision, tt.bumpRevision, result, tt.expected)
			}
		})
	}
}
