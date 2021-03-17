package metrics

import (
	"bytes"
	"testing"
)

func TestGetPageCacheRSSFromSmapsFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		bb := bytes.NewBufferString(s)
		_, _, err := getRSSStatsFromSmaps(bb)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f("foobar")

	// Invalid unit for Rss
	f(`7ffcdf335000-7ffcdf337000 r-xp 00000000 00:00 0                          [vdso]
Size:                 80 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                  12 MB
Pss:                   0 kB
Shared_Clean:          4 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            4 kB
Anonymous:             0 kB
LazyFree:              0 kB
AnonHugePages:         0 kB
ShmemPmdMapped:        0 kB
Shared_Hugetlb:        0 kB
Private_Hugetlb:       0 kB
Swap:                  0 kB
SwapPss:               0 kB
Locked:                0 kB
VmFlags: rd ex mr mw me de sd 
`)

	// Invalid unit for Anonymous
	f(`7ffcdf335000-7ffcdf337000 r-xp 00000000 00:00 0                          [vdso]
Size:                 80 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                  12 kB
Pss:                   0 kB
Shared_Clean:          4 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            4 kB
Anonymous:             5 MB
LazyFree:              0 kB
AnonHugePages:         0 kB
ShmemPmdMapped:        0 kB
Shared_Hugetlb:        0 kB
Private_Hugetlb:       0 kB
Swap:                  0 kB
SwapPss:               0 kB
Locked:                0 kB
VmFlags: rd ex mr mw me de sd 
`)

	// Invalid size for Rss
	f(`7ffcdf335000-7ffcdf337000 r-xp 00000000 00:00 0                          [vdso]
Size:                 80 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                 1.2 kB
Pss:                   0 kB
Shared_Clean:          4 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            4 kB
Anonymous:             0 kB
LazyFree:              0 kB
AnonHugePages:         0 kB
ShmemPmdMapped:        0 kB
Shared_Hugetlb:        0 kB
Private_Hugetlb:       0 kB
Swap:                  0 kB
SwapPss:               0 kB
Locked:                0 kB
VmFlags: rd ex mr mw me de sd 
`)

	// Too big size for Rss
	f(`7ffcdf335000-7ffcdf337000 r-xp 00000000 00:00 0                          [vdso]
Size:                 80 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss: 9999999999999999999 kB
Pss:                   0 kB
Shared_Clean:          4 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            4 kB
Anonymous:             0 kB
LazyFree:              0 kB
AnonHugePages:         0 kB
ShmemPmdMapped:        0 kB
Shared_Hugetlb:        0 kB
Private_Hugetlb:       0 kB
Swap:                  0 kB
SwapPss:               0 kB
Locked:                0 kB
VmFlags: rd ex mr mw me de sd 
`)

	// Partial entry
	f(`7ffcdf335000-7ffcdf337000 r-xp 00000000 00:00 0                          [vdso]
Size:                 80 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                  12 kB
Pss:                   0 kB
Shared_Clean:          4 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            4 kB
Anonymous:             0 kB
LazyFree:              0 kB
`)

	// Partial second entry
	f(`7ffcdf335000-7ffcdf337000 r-xp 00000000 00:00 0                          [vdso]
Size:                 80 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                  12 kB
Pss:                   0 kB
Shared_Clean:          4 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            4 kB
Anonymous:             0 kB
LazyFree:              0 kB
AnonHugePages:         0 kB
ShmemPmdMapped:        0 kB
Shared_Hugetlb:        0 kB
Private_Hugetlb:       0 kB
Swap:                  0 kB
SwapPss:               0 kB
Locked:                0 kB
VmFlags: rd ex mr mw me de sd 
ffffffffff600000-ffffffffff601000 r-xp 00000000 00:00 0                  [vsyscall]
Size:               1024 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                 120 kB
`)
}

func TestGetPageCacheRSSFromSmapsSuccess(t *testing.T) {
	s := `7ffcdf335000-7ffcdf337000 r-xp 00000000 00:00 0                          [vdso]
Size:                 80 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                  12 kB
Pss:                   0 kB
Shared_Clean:          4 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            4 kB
Anonymous:             0 kB
LazyFree:              0 kB
AnonHugePages:         0 kB
ShmemPmdMapped:        0 kB
Shared_Hugetlb:        0 kB
Private_Hugetlb:       0 kB
Swap:                  0 kB
SwapPss:               0 kB
Locked:                0 kB
VmFlags: rd ex mr mw me de sd 
ffffffffff600000-ffffffffff601000 r-xp 00000000 00:00 0                  [vsyscall]
Size:               1024 kB
KernelPageSize:        4 kB
MMUPageSize:           4 kB
Rss:                 120 kB
Pss:                   0 kB
Shared_Clean:          0 kB
Shared_Dirty:          0 kB
Private_Clean:         0 kB
Private_Dirty:         0 kB
Referenced:            0 kB
Anonymous:          1024 kB
LazyFree:              0 kB
AnonHugePages:         0 kB
ShmemPmdMapped:        0 kB
Shared_Hugetlb:        0 kB
Private_Hugetlb:       0 kB
Swap:                  0 kB
SwapPss:               0 kB
Locked:                0 kB
VmFlags: rd ex 
`
	bb := bytes.NewBufferString(s)
	pageCache, anonymous, err := getRSSStatsFromSmaps(bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedPageCache := uint64(12 * 1024)
	if pageCache != expectedPageCache {
		t.Fatalf("unexpected page cache rss; got %d; want %d", pageCache, expectedPageCache)
	}
	expectedAnonymous := uint64(120 * 1024)
	if anonymous != expectedAnonymous {
		t.Fatalf("unexpected anonymous rss; got %d; want %d", anonymous, expectedAnonymous)
	}
}

func TestGetMaxFilesLimit(t *testing.T) {
	f := func(want uint64, path string, wantErr bool) {
		t.Helper()
		got, err := getMaxFilesLimit(path)
		if err != nil && !wantErr {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected result: %d, want: %d at getMaxFilesLimit", got, want)
		}

	}
	f(1024, "testdata/limits", false)
	f(0, "testdata/bad_path", true)
	f(0, "testdata/limits_bad", true)
}

func TestGetOpenFDsCount(t *testing.T) {
	f := func(want uint64, path string, wantErr bool) {
		t.Helper()
		got, err := getOpenFDsCount(path)
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected result: %d, want: %d at getOpenFDsCount", got, want)
		}
	}
	f(5, "testdata/fd/", false)
	f(0, "testdata/fd/0", true)
	f(0, "testdata/limits", true)
}

func TestGetMemStats(t *testing.T) {
	f := func(want memStats, path string, wantErr bool) {
		t.Helper()
		got, err := getMemStats(path)
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil && *got != want {
			t.Fatalf("unexpected result: %d, want: %d at getMemStats", *got, want)
		}
	}
	f(memStats{vmPeak: 2130489344, rssPeak: 200679424, rssAnon: 121602048, rssFile: 11362304}, "testdata/status", false)
	f(memStats{}, "testdata/status_bad", true)
}
