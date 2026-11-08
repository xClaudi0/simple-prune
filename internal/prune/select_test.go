package prune

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeDaemon struct {
	used      []string
	images    []Image // returned when !danglingOnly
	dangling  []Image // returned when danglingOnly
	usedErr   error
	imagesErr error
}

func (f fakeDaemon) ContainerImageIDs(context.Context) ([]string, error) { return f.used, f.usedErr }
func (f fakeDaemon) Images(_ context.Context, danglingOnly bool) ([]Image, error) {
	if f.imagesErr != nil {
		return nil, f.imagesErr
	}
	if danglingOnly {
		return f.dangling, nil
	}
	return f.images, nil
}

func img(id string, size int64, tags ...string) Image {
	return Image{ID: id, Tags: tags, Size: size, Created: time.Unix(0, 0).UTC()}
}

func ids(r Result) []string {
	out := []string{}
	for _, i := range r.Images {
		out = append(out, i.ID)
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStoppedContainerImageExcluded(t *testing.T) {
	d := fakeDaemon{
		used:     []string{"sha256:used"}, // e.g. from a stopped container
		dangling: []Image{img("sha256:used", 10), img("sha256:free", 20)},
	}
	r, err := Select(context.Background(), d, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(r); !eq(got, []string{"sha256:free"}) {
		t.Fatalf("got %v", got)
	}
	if r.TotalSize != 20 {
		t.Fatalf("total %d", r.TotalSize)
	}
}

func TestMultiTagImageSingleEntry(t *testing.T) {
	// Even if a daemon returned one summary per tag, same ID must collapse.
	d := fakeDaemon{images: []Image{
		img("sha256:a", 100, "a:1"),
		img("sha256:a", 100, "a:2"),
		img("sha256:b", 50, "b:1", "b:2"),
	}}
	r, _ := Select(context.Background(), d, true)
	if got := ids(r); !eq(got, []string{"sha256:a", "sha256:b"}) {
		t.Fatalf("got %v", got)
	}
	if r.TotalSize != 150 {
		t.Fatalf("total %d (dup counted?)", r.TotalSize)
	}
	if len(r.Images[1].Tags) != 2 {
		t.Fatalf("tags %v", r.Images[1].Tags)
	}
}

func TestDanglingVsAll(t *testing.T) {
	d := fakeDaemon{
		dangling: []Image{img("sha256:d", 5)},
		images:   []Image{img("sha256:d", 5), img("sha256:t", 9, "t:1")},
	}
	base, _ := Select(context.Background(), d, false)
	all, _ := Select(context.Background(), d, true)
	if base.Mode != "prune" || !eq(ids(base), []string{"sha256:d"}) {
		t.Fatalf("base %v %s", ids(base), base.Mode)
	}
	if all.Mode != "prune -a" || !eq(ids(all), []string{"sha256:t", "sha256:d"}) {
		t.Fatalf("all %v %s", ids(all), all.Mode)
	}
}

func TestEmpty(t *testing.T) {
	r, err := Select(context.Background(), fakeDaemon{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.Images == nil || len(r.Images) != 0 || r.TotalSize != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestAllImagesUsed(t *testing.T) {
	d := fakeDaemon{used: []string{"sha256:a"}, images: []Image{img("sha256:a", 1, "a:1")}}
	r, _ := Select(context.Background(), d, true)
	if len(r.Images) != 0 {
		t.Fatalf("%v", ids(r))
	}
}

func TestSortBySizeDesc(t *testing.T) {
	d := fakeDaemon{images: []Image{img("sha256:s", 1), img("sha256:l", 3), img("sha256:m", 2)}}
	r, _ := Select(context.Background(), d, true)
	if got := ids(r); !eq(got, []string{"sha256:l", "sha256:m", "sha256:s"}) {
		t.Fatalf("got %v", got)
	}
}

func TestErrorsPropagate(t *testing.T) {
	boom := errors.New("boom")
	if _, err := Select(context.Background(), fakeDaemon{usedErr: boom}, false); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := Select(context.Background(), fakeDaemon{imagesErr: boom}, false); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
