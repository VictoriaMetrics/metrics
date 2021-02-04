package metrics

import "testing"

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
