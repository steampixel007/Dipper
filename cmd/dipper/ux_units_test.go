package main

import "testing"

func TestUXUploadLimitsNameTheirActualByteUnits(t *testing.T) {
	for _, sample := range []struct {
		bytes int64
		label string
	}{
		{256 * 1024, "256 KiB/s"},
		{512 * 1024, "512 KiB/s"},
		{1024 * 1024, "1 MiB/s"},
		{2 * 1024 * 1024, "2 MiB/s"},
	} {
		if got := uploadLabel(sample.bytes); got != sample.label {
			t.Errorf("%d bytes/s displayed as %q; want %q", sample.bytes, got, sample.label)
		}
		if got := uploadLimit(sample.label); got != sample.bytes {
			t.Errorf("%q applies %d bytes/s; want %d", sample.label, got, sample.bytes)
		}
	}
	for _, sample := range []struct {
		legacy string
		bytes  int64
	}{
		{"256 KB/s", 256 * 1024},
		{"512 KB/s", 512 * 1024},
		{"1 MB/s", 1024 * 1024},
		{"2 MB/s", 2 * 1024 * 1024},
	} {
		if got := uploadLimit(sample.legacy); got != sample.bytes {
			t.Errorf("legacy preference %q changed its cap from %d to %d", sample.legacy, sample.bytes, got)
		}
	}
}
