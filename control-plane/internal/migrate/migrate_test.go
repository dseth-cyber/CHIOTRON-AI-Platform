package migrate

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadOrdersByVersion(t *testing.T) {
	files := fstest.MapFS{
		"0010_later.sql":  {Data: []byte("SELECT 10;")},
		"0002_second.sql": {Data: []byte("SELECT 2;")},
		"0001_first.sql":  {Data: []byte("SELECT 1;")},
		"README.md":       {Data: []byte("not a migration")},
	}

	loaded, err := load(files, ".")
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("load() returned %d migrations, want 3 (non-SQL files ignored)", len(loaded))
	}

	wantVersions := []int64{1, 2, 10}
	wantNames := []string{"first", "second", "later"}
	for i, m := range loaded {
		if m.version != wantVersions[i] {
			t.Errorf("migration %d version = %d, want %d", i, m.version, wantVersions[i])
		}
		if m.name != wantNames[i] {
			t.Errorf("migration %d name = %q, want %q", i, m.name, wantNames[i])
		}
		if m.checksum == "" {
			t.Errorf("migration %d has an empty checksum", i)
		}
	}
}

// The checksum is what makes an edit to an applied migration detectable, so it
// must depend on content and nothing else.
func TestLoadChecksumTracksContent(t *testing.T) {
	before, err := load(fstest.MapFS{"0001_init.sql": {Data: []byte("SELECT 1;")}}, ".")
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}
	same, err := load(fstest.MapFS{"0001_init.sql": {Data: []byte("SELECT 1;")}}, ".")
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}
	changed, err := load(fstest.MapFS{"0001_init.sql": {Data: []byte("SELECT 2;")}}, ".")
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}

	if before[0].checksum != same[0].checksum {
		t.Error("identical content produced different checksums")
	}
	if before[0].checksum == changed[0].checksum {
		t.Error("changed content produced the same checksum; an edited migration would go unnoticed")
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		files fstest.MapFS
		want  string
	}{
		"missing description": {
			files: fstest.MapFS{"0001.sql": {Data: []byte("SELECT 1;")}},
			want:  "NNNN_description.sql",
		},
		"non-numeric version": {
			files: fstest.MapFS{"init_tables.sql": {Data: []byte("SELECT 1;")}},
			want:  "numeric version",
		},
		"duplicate version": {
			files: fstest.MapFS{
				"0001_first.sql":  {Data: []byte("SELECT 1;")},
				"1_duplicate.sql": {Data: []byte("SELECT 2;")},
			},
			want: "duplicate migration version",
		},
		"no migrations": {
			files: fstest.MapFS{"README.md": {Data: []byte("nothing here")}},
			want:  "no migrations found",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(tc.files, ".")
			if err == nil {
				t.Fatal("load() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
