// Package prune computes which images `docker image prune [-a]` would remove.
// It is strictly read-only: the Daemon interface exposes list operations only.
package prune

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Image is the subset of image metadata needed for the preview.
type Image struct {
	ID      string // full ID, "sha256:..."
	Tags    []string
	Created time.Time
	Size    int64
	// SharedSize is the size of layers shared with other images, or -1 if the
	// daemon did not report it.
	SharedSize int64
}

// UniqueSize returns the size used only by this image (Size - SharedSize) and
// whether it is known.
func (i Image) UniqueSize() (int64, bool) {
	if i.SharedSize < 0 || i.SharedSize > i.Size {
		return 0, false
	}
	return i.Size - i.SharedSize, true
}

// Daemon is the minimal read-only view of the Docker daemon required here.
type Daemon interface {
	// ContainerImageIDs returns the full image IDs used by ALL containers,
	// including stopped ones.
	ContainerImageIDs(ctx context.Context) ([]string, error)
	// Images lists images. If danglingOnly is true, only dangling images are
	// returned (daemon-side filter dangling=true).
	// filters are extra daemon-side image filters (until, label, label!).
	Images(ctx context.Context, danglingOnly bool, filters []Filter) ([]Image, error)
}

// Filter is a prune-style filter such as until=24h or label=env=dev.
type Filter struct{ Key, Value string }

// ParseFilters parses `--filter` values like docker prune does:
// "until=<duration|timestamp>", "label=<key>[=<value>]" and "label!=<key>[=<value>]".
func ParseFilters(raw []string) ([]Filter, error) {
	out := make([]Filter, 0, len(raw))
	for _, r := range raw {
		key, val, ok := strings.Cut(r, "=")
		if !ok || val == "" {
			return nil, fmt.Errorf("invalid filter %q: expected key=value", r)
		}
		switch key {
		case "until", "label", "label!":
		default:
			return nil, fmt.Errorf("unsupported filter %q: supported filters are until, label, label!", key)
		}
		out = append(out, Filter{Key: key, Value: val})
	}
	return out, nil
}

// Result is the outcome of a simulated prune.
type Result struct {
	Mode      string // "prune" or "prune -a"
	Images    []Image
	TotalSize int64
	// MinReclaimable is a lower bound of the space that is certainly freed: the
	// sum of the sizes used only by the selected images. Valid only if
	// MinKnown is true (every image reported its shared size).
	MinReclaimable int64
	MinKnown       bool
}

// Select returns the images that `docker image prune` (all=false) or
// `docker image prune -a` (all=true) would remove. Images are compared by ID
// only, deduplicated by ID, and sorted by size descending (ties by ID).
func Select(ctx context.Context, d Daemon, all bool, filters ...Filter) (Result, error) {
	used, err := d.ContainerImageIDs(ctx)
	if err != nil {
		return Result{}, err
	}
	excluded := make(map[string]struct{}, len(used))
	for _, id := range used {
		excluded[id] = struct{}{}
	}

	candidates, err := d.Images(ctx, !all, filters)
	if err != nil {
		return Result{}, err
	}
	if !all {
		// With the containerd image store the same ID may have both a tagged
		// and a dangling record, and the daemon's dangling filter returns the
		// latter. An image that still has a tag is not removed by prune, so
		// drop every ID that carries a tag in the full list.
		full, err := d.Images(ctx, false, nil)
		if err != nil {
			return Result{}, err
		}
		for _, img := range full {
			if len(img.Tags) > 0 {
				excluded[img.ID] = struct{}{}
			}
		}
	}

	res := Result{Mode: "prune", Images: []Image{}}
	if all {
		res.Mode = "prune -a"
	}
	res.MinKnown = true
	seen := make(map[string]struct{}, len(candidates))
	for _, img := range candidates {
		if _, ok := excluded[img.ID]; ok {
			continue
		}
		if _, ok := seen[img.ID]; ok {
			continue
		}
		seen[img.ID] = struct{}{}
		if img.Tags == nil {
			img.Tags = []string{}
		}
		res.Images = append(res.Images, img)
		res.TotalSize += img.Size
		if u, ok := img.UniqueSize(); ok {
			res.MinReclaimable += u
		} else {
			res.MinKnown = false
		}
	}
	if len(res.Images) == 0 {
		res.MinKnown = false
	}
	sort.SliceStable(res.Images, func(i, j int) bool {
		if res.Images[i].Size != res.Images[j].Size {
			return res.Images[i].Size > res.Images[j].Size
		}
		return res.Images[i].ID < res.Images[j].ID
	})
	return res, nil
}
