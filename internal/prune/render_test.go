package prune

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sample() Result {
	t := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return Result{Mode: "prune -a", TotalSize: 1_500_000, Images: []Image{
		{ID: "sha256:0123456789abcdef0123", Tags: []string{"a:1", "a:2"}, Created: t, Size: 1_000_000},
		{ID: "sha256:fedcba9876543210ffff", Tags: []string{}, Created: t, Size: 500_000},
	}}
}

func TestTable(t *testing.T) {
	var b bytes.Buffer
	if err := WriteTable(&b, sample()); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	for _, want := range []string{"IMAGE ID", "0123456789ab", "a:1, a:2", "<none>:<none>",
		"2026-01-02 03:04:05", "1MB", "500kB",
		"2 images would be removed by 'docker image prune -a' — total 1.5MB", "overestimate"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "sha256:") {
		t.Errorf("prefix leaked:\n%s", s)
	}
}

func TestTableMinReclaimable(t *testing.T) {
	r := sample()
	r.MinKnown, r.MinReclaimable = true, 900_000
	var b bytes.Buffer
	_ = WriteTable(&b, r)
	if !strings.Contains(b.String(), "at least 900kB is used only by these images") {
		t.Fatalf("missing note:\n%s", b.String())
	}
	var j bytes.Buffer
	_ = WriteJSON(&j, r)
	if !strings.Contains(j.String(), `"min_reclaimable": 900000`) || !strings.Contains(j.String(), `"unique_size"`) {
		t.Fatalf("json:\n%s", j.String())
	}
}

func TestTableEmpty(t *testing.T) {
	var b bytes.Buffer
	_ = WriteTable(&b, Result{Mode: "prune", Images: []Image{}})
	if got, want := b.String(), "No images would be removed by 'docker image prune'.\n"; got != want {
		t.Fatalf("got %q", got)
	}
}

func TestJSON(t *testing.T) {
	var b bytes.Buffer
	_ = WriteJSON(&b, sample())
	var o struct {
		Mode      string `json:"mode"`
		Count     int    `json:"count"`
		TotalSize int64  `json:"total_size"`
		Images    []struct {
			ID      string   `json:"id"`
			ShortID string   `json:"short_id"`
			Created string   `json:"created"`
			Tags    []string `json:"tags"`
			Size    int64    `json:"size"`
		} `json:"images"`
	}
	if err := json.Unmarshal(b.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	if o.Mode != "prune -a" || o.Count != 2 || o.TotalSize != 1_500_000 {
		t.Fatalf("%+v", o)
	}
	if o.Images[0].Created != "2026-01-02T03:04:05Z" || o.Images[0].ShortID != "0123456789ab" {
		t.Fatalf("%+v", o.Images[0])
	}
}

func TestJSONEmpty(t *testing.T) {
	var b bytes.Buffer
	_ = WriteJSON(&b, Result{Mode: "prune"})
	s := b.String()
	for _, want := range []string{`"count": 0`, `"total_size": 0`, `"images": []`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
}
