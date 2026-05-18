package snapshot

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
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

func TestReadSnapshotPreamble(t *testing.T) {
	requestKey := RequestStoreKey + "/" + APIVersion

	t.Run("metadata first", func(t *testing.T) {
		tarReader := newTestTarReader(t,
			testTarEntry{name: SnapshotMetadataKey, value: mustMarshalSnapshotMetadata(t, EtcdSnapshotKind)},
			testTarEntry{name: requestKey, value: []byte("request")},
		)

		kind, pendingEntryName, hasPendingEntry, err := readSnapshotPreamble(tarReader)
		if err != nil {
			t.Fatalf("readSnapshotPreamble() error = %v", err)
		}
		if kind != EtcdSnapshotKind {
			t.Fatalf("kind = %s; want %s", kind, EtcdSnapshotKind)
		}
		if hasPendingEntry {
			t.Fatalf("pending entry = %q; want none", pendingEntryName)
		}

		key, value, err := readArchiveEntry(tarReader)
		if err != nil {
			t.Fatalf("readArchiveEntry() error = %v", err)
		}
		if string(key) != requestKey || string(value) != "request" {
			t.Fatalf("next entry = %q/%q; want %q/request", string(key), string(value), requestKey)
		}
	})

	t.Run("release before metadata", func(t *testing.T) {
		tarReader := newTestTarReader(t,
			testTarEntry{name: SnapshotReleaseKey, value: []byte("release")},
			testTarEntry{name: SnapshotMetadataKey, value: mustMarshalSnapshotMetadata(t, KeyValueSnapshotKind)},
			testTarEntry{name: requestKey, value: []byte("request")},
		)

		kind, pendingEntryName, hasPendingEntry, err := readSnapshotPreamble(tarReader)
		if err != nil {
			t.Fatalf("readSnapshotPreamble() error = %v", err)
		}
		if kind != KeyValueSnapshotKind {
			t.Fatalf("kind = %s; want %s", kind, KeyValueSnapshotKind)
		}
		if hasPendingEntry {
			t.Fatalf("pending entry = %q; want none", pendingEntryName)
		}

		key, value, err := readArchiveEntry(tarReader)
		if err != nil {
			t.Fatalf("readArchiveEntry() error = %v", err)
		}
		if string(key) != requestKey || string(value) != "request" {
			t.Fatalf("next entry = %q/%q; want %q/request", string(key), string(value), requestKey)
		}
	})

	t.Run("legacy release and request", func(t *testing.T) {
		tarReader := newTestTarReader(t,
			testTarEntry{name: SnapshotReleaseKey, value: []byte("release")},
			testTarEntry{name: requestKey, value: []byte("request")},
			testTarEntry{name: "/registry/test", value: []byte("value")},
		)

		kind, pendingEntryName, hasPendingEntry, err := readSnapshotPreamble(tarReader)
		if err != nil {
			t.Fatalf("readSnapshotPreamble() error = %v", err)
		}
		if kind != KeyValueSnapshotKind {
			t.Fatalf("kind = %s; want %s", kind, KeyValueSnapshotKind)
		}
		if !hasPendingEntry {
			t.Fatalf("pending entry missing; want %s", requestKey)
		}
		if pendingEntryName != requestKey {
			t.Fatalf("pending entry = %q; want unread request", pendingEntryName)
		}

		requestValue, err := readArchiveEntryValue(tarReader)
		if err != nil {
			t.Fatalf("readArchiveEntryValue() error = %v", err)
		}
		if string(requestValue) != "request" {
			t.Fatalf("request value = %q; want request", string(requestValue))
		}
		key, value, err := readArchiveEntry(tarReader)
		if err != nil {
			t.Fatalf("readArchiveEntry() error = %v", err)
		}
		if string(key) != "/registry/test" || string(value) != "value" {
			t.Fatalf("next entry = %q/%q; want /registry/test/value", string(key), string(value))
		}
	})

	t.Run("legacy key first", func(t *testing.T) {
		tarReader := newTestTarReader(t,
			testTarEntry{name: "/registry/test", value: []byte("value")},
		)

		kind, pendingEntryName, hasPendingEntry, err := readSnapshotPreamble(tarReader)
		if err != nil {
			t.Fatalf("readSnapshotPreamble() error = %v", err)
		}
		if kind != KeyValueSnapshotKind {
			t.Fatalf("kind = %s; want %s", kind, KeyValueSnapshotKind)
		}
		if !hasPendingEntry || pendingEntryName != "/registry/test" {
			t.Fatalf("pending entry = %q/%t; want unread /registry/test", pendingEntryName, hasPendingEntry)
		}

		value, err := readArchiveEntryValue(tarReader)
		if err != nil {
			t.Fatalf("readArchiveEntryValue() error = %v", err)
		}
		if string(value) != "value" {
			t.Fatalf("value = %q; want value", string(value))
		}
		if _, _, err := readArchiveEntry(tarReader); err != io.EOF {
			t.Fatalf("readArchiveEntry() error = %v; want EOF", err)
		}
	})
}

type testTarEntry struct {
	name  string
	value []byte
}

func newTestTarReader(t *testing.T, entries ...testTarEntry) *tar.Reader {
	t.Helper()

	buf := &bytes.Buffer{}
	tarWriter := tar.NewWriter(buf)
	for _, entry := range entries {
		if err := tarWriter.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     entry.name,
			Size:     int64(len(entry.value)),
			Mode:     0666,
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(entry.value); err != nil {
			t.Fatalf("write tar entry: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	return tar.NewReader(buf)
}

func mustMarshalSnapshotMetadata(t *testing.T, kind SnapshotKind) []byte {
	t.Helper()

	metadataBytes, err := json.Marshal(SnapshotMetadata{
		Kind: kind,
	})
	if err != nil {
		t.Fatalf("marshal snapshot metadata: %v", err)
	}

	return metadataBytes
}
