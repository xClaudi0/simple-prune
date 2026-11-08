// Package prune computes which images `docker image prune [-a]` would remove.
// It is strictly read-only: the Daemon interface exposes list operations only.
package prune

import (
	"context"
	"sort"
	"time"
)

// Image is the subset of image metadata needed for the preview.
type Image struct {
	ID      string // full ID, "sha256:..."
	Tags    []string
	Created time.Time
	Size    int64
}

// Daemon is the minimal read-only view of the Docker daemon required here.
type Daemon interface {
	// ContainerImageIDs returns the full image IDs used by ALL containers,
	// including stopped ones.
	ContainerImageIDs(ctx context.Context) ([]string, error)
	// Images lists images. If danglingOnly is true, only dangling images are
	// returned (daemon-side filter dangling=true).
	Images(ctx context.Context, danglingOnly bool) ([]Image, error)
}

// Result is the outcome of a simulated prune.
type Result struct {
	Mode      string // "prune" or "prune -a"
	Images    []Image
	TotalSize int64
}

// Select returns the images that `docker image prune` (all=false) or
// `docker image prune -a` (all=true) would remove. Images are compared by ID
// only, deduplicated by ID, and sorted by size descending (ties by ID).
func Select(ctx context.Context, d Daemon, all bool) (Result, error) {
	used, err := d.ContainerImageIDs(ctx)
	if err != nil {
		return Result{}, err
	}
	usedSet := make(map[string]struct{}, len(used))
	for _, id := range used {
		usedSet[id] = struct{}{}
	}

	candidates, err := d.Images(ctx, !all)
	if err != nil {
		return Result{}, err
	}

	res := Result{Mode: "prune", Images: []Image{}}
	if all {
		res.Mode = "prune -a"
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, img := range candidates {
		if _, ok := usedSet[img.ID]; ok {
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
	}
	sort.SliceStable(res.Images, func(i, j int) bool {
		if res.Images[i].Size != res.Images[j].Size {
			return res.Images[i].Size > res.Images[j].Size
		}
		return res.Images[i].ID < res.Images[j].ID
	})
	return res, nil
}
