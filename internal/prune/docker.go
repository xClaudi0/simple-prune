package prune

import (
	"context"
	"time"

	"github.com/moby/moby/client"
)

// apiLister is the read-only subset of the Moby client used here.
// It deliberately excludes every remove/prune call.
type apiLister interface {
	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	ImageList(ctx context.Context, options client.ImageListOptions) (client.ImageListResult, error)
}

// NewDaemon adapts a Moby API client to the Daemon interface.
func NewDaemon(api apiLister) Daemon { return dockerDaemon{api: api} }

type dockerDaemon struct{ api apiLister }

func (d dockerDaemon) ContainerImageIDs(ctx context.Context) ([]string, error) {
	res, err := d.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(res.Items))
	for _, c := range res.Items {
		ids = append(ids, c.ImageID)
	}
	return ids, nil
}

func (d dockerDaemon) Images(ctx context.Context, danglingOnly bool, filters []Filter) ([]Image, error) {
	opts := client.ImageListOptions{Filters: make(client.Filters), SharedSize: true}
	if danglingOnly {
		opts.Filters = opts.Filters.Add("dangling", "true")
	}
	for _, f := range filters {
		opts.Filters = opts.Filters.Add(f.Key, f.Value)
	}
	res, err := d.api.ImageList(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Image, 0, len(res.Items))
	for _, s := range res.Items {
		tags := make([]string, 0, len(s.RepoTags))
		for _, t := range s.RepoTags {
			if t != "<none>:<none>" {
				tags = append(tags, t)
			}
		}
		out = append(out, Image{ID: s.ID, Tags: tags, Created: time.Unix(s.Created, 0).UTC(), Size: s.Size, SharedSize: s.SharedSize})
	}
	return out, nil
}
