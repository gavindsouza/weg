package remote

import (
	"io"
	"strings"
	"testing"
)

func TestMergeVersions_GlobalOrder(t *testing.T) {
	// Two already-sorted streams; merge must interleave by creation, then name.
	a := strings.NewReader(
		`{"name":"a1","creation":"2024-01-01 00:00:00"}` + "\n" +
			`{"name":"a2","creation":"2024-01-03 00:00:00"}` + "\n")
	b := strings.NewReader(
		`{"name":"b1","creation":"2024-01-02 00:00:00"}` + "\n" +
			`{"name":"b2","creation":"2024-01-03 00:00:00"}` + "\n")

	var got []string
	err := mergeVersions([]io.Reader{a, b}, func(v VersionRecord) error {
		got = append(got, v.Name)
		return nil
	})
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	// 2024-01-03 tie broken by name: a2 before b2.
	want := []string{"a1", "b1", "a2", "b2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestMergeVersions_EmptyAndSingle(t *testing.T) {
	var got []string
	err := mergeVersions(
		[]io.Reader{strings.NewReader(""), strings.NewReader(`{"name":"x","creation":"2024-01-01 00:00:00"}` + "\n")},
		func(v VersionRecord) error { got = append(got, v.Name); return nil },
	)
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	if len(got) != 1 || got[0] != "x" {
		t.Fatalf("got %v", got)
	}
}

// A truncated trailing line drains the other stream fully, then surfaces the error.
func TestMergeVersions_TruncatedStream(t *testing.T) {
	good := strings.NewReader(`{"name":"g1","creation":"2024-01-01 00:00:00"}` + "\n")
	bad := strings.NewReader(`{"name":"b1","creation":"2024-01-02 00:00:00"}` + "\n" + `{"name":"b2","creat`)

	var got []string
	err := mergeVersions([]io.Reader{good, bad}, func(v VersionRecord) error {
		got = append(got, v.Name)
		return nil
	})
	if err == nil {
		t.Fatal("expected decode error from truncated stream")
	}
	// Both complete records still delivered before the error surfaced.
	if strings.Join(got, ",") != "g1,b1" {
		t.Fatalf("got %v, want [g1 b1]", got)
	}
}
