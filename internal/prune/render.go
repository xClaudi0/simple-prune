package prune

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	units "github.com/docker/go-units"
)

const timeLayout = "2006-01-02 15:04:05"

// ShortID returns the first 12 hex chars of an image ID, without "sha256:".
func ShortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		id = id[:12]
	}
	return id
}

// HumanSize formats bytes in decimal units (kB/MB/GB) like the Docker CLI.
func HumanSize(n int64) string {
	return units.HumanSizeWithPrecision(float64(n), 3)
}

// WriteTable renders the human-readable table. Times are UTC.
func WriteTable(w io.Writer, r Result) error {
	cmd := "docker image " + r.Mode
	if len(r.Images) == 0 {
		_, err := fmt.Fprintf(w, "No images would be removed by '%s'.\n", cmd)
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "IMAGE ID\tTAGS\tCREATED\tSIZE")
	for _, img := range r.Images {
		tags := "<none>:<none>"
		if len(img.Tags) > 0 {
			tags = strings.Join(img.Tags, ", ")
		}
		created := "-" // daemon reports Created=0 for some images (no creation data)
		if img.Created.Unix() > 0 {
			created = img.Created.UTC().Format(timeLayout)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ShortID(img.ID), tags, created, HumanSize(img.Size))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	noun := "images"
	if len(r.Images) == 1 {
		noun = "image"
	}
	_, err := fmt.Fprintf(w, "\n%d %s would be removed by '%s' — total %s\n"+
		"(total is an overestimate: layers shared between images are counted more than once)\n",
		len(r.Images), noun, cmd, HumanSize(r.TotalSize))
	return err
}

type jsonImage struct {
	ID      string   `json:"id"`
	ShortID string   `json:"short_id"`
	Tags    []string `json:"tags"`
	Created string   `json:"created"`
	Size    int64    `json:"size"`
}

type jsonOutput struct {
	Mode      string      `json:"mode"`
	Count     int         `json:"count"`
	TotalSize int64       `json:"total_size"`
	Images    []jsonImage `json:"images"`
}

// WriteJSON renders the machine-readable output.
func WriteJSON(w io.Writer, r Result) error {
	out := jsonOutput{Mode: r.Mode, Count: len(r.Images), TotalSize: r.TotalSize, Images: []jsonImage{}}
	for _, img := range r.Images {
		tags := img.Tags
		if tags == nil {
			tags = []string{}
		}
		out.Images = append(out.Images, jsonImage{
			ID: img.ID, ShortID: ShortID(img.ID), Tags: tags,
			Created: img.Created.UTC().Format(time.RFC3339), Size: img.Size,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
