package epub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EditOptions struct {
	OutPath        string
	NavReplacePath string
	DumpNavPath    string
	DumpMetaPath   string
	MetadataPatch  MetadataPatch
	TouchModified  bool
}

type MetadataPatch struct {
	Title       *string   `json:"title,omitempty"`
	Language    *string   `json:"language,omitempty"`
	Identifier  *string   `json:"identifier,omitempty"`
	Description *string   `json:"description,omitempty"`
	Creators    *[]string `json:"creators,omitempty"`
}

type MetadataSnapshot struct {
	Title       string   `json:"title,omitempty"`
	Language    string   `json:"language,omitempty"`
	Identifier  string   `json:"identifier,omitempty"`
	Description string   `json:"description,omitempty"`
	Creators    []string `json:"creators,omitempty"`
}

func (p MetadataPatch) IsZero() bool {
	return p.Title == nil &&
		p.Language == nil &&
		p.Identifier == nil &&
		p.Description == nil &&
		p.Creators == nil
}

func EditEPUB(ctx context.Context, input string, opts EditOptions) error {
	if input == "" {
		return fmt.Errorf("input EPUB path is required")
	}

	vol, err := loadVolume(ctx, 0, input)
	if err != nil {
		return err
	}
	defer os.RemoveAll(vol.TempDir)

	pkg := vol.PackageDoc

	if opts.DumpMetaPath != "" {
		if err := writeMetadataSnapshot(pkg.Metadata, opts.DumpMetaPath); err != nil {
			return err
		}
	}

	if opts.DumpNavPath != "" {
		if err := dumpNavFile(vol, opts.DumpNavPath); err != nil {
			return err
		}
	}

	metaChanged := false
	if !opts.MetadataPatch.IsZero() {
		metaChanged = applyMetadataPatch(&pkg.Metadata, opts.MetadataPatch)
	}

	navChanged := false
	if opts.NavReplacePath != "" {
		if vol.NavHref == "" {
			return fmt.Errorf("nav document not found in %s", input)
		}
		if err := replaceNavFile(vol, opts.NavReplacePath); err != nil {
			return err
		}
		navChanged = true
	}

	needsWrite := metaChanged || navChanged
	if !needsWrite {
		return nil
	}

	if opts.TouchModified {
		updateModifiedTimestamp(&pkg.Metadata)
	}

	if err := writePackage(pkg, vol.PackagePath); err != nil {
		return err
	}

	outPath := opts.OutPath
	if outPath == "" {
		outPath = input
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(outPath), "novfmt-edit-*.epub")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	if err := writeZip(vol.RootDir, tmpPath); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		return err
	}
	tmpPath = ""

	return nil
}

func writeMetadataSnapshot(meta Metadata, dest string) error {
	snapshot := MetadataSnapshot{
		Title:       firstDCValue(meta.Titles),
		Language:    firstDCValue(meta.Languages),
		Identifier:  firstDCValue(meta.Identifiers),
		Description: firstDCValue(meta.Descriptions),
		Creators:    collectCreators(meta.Creators),
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := ensureParentDir(dest); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func dumpNavFile(vol *Volume, dest string) error {
	if vol.NavHref == "" {
		return fmt.Errorf("nav document not found")
	}
	src := filepath.Join(filepath.Dir(vol.PackagePath), filepath.FromSlash(vol.NavHref))
	if err := ensureParentDir(dest); err != nil {
		return err
	}
	return copyFile(src, dest, 0o644)
}

func replaceNavFile(vol *Volume, src string) error {
	target := filepath.Join(filepath.Dir(vol.PackagePath), filepath.FromSlash(vol.NavHref))
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	mode := info.Mode()
	return copyFile(src, target, mode)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func firstDCValue(nodes []DCMeta) string {
	if len(nodes) == 0 {
		return ""
	}
	return nodes[0].Value
}

func collectCreators(nodes []DCMeta) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if strings.TrimSpace(n.Value) == "" {
			continue
		}
		out = append(out, n.Value)
	}
	return out
}

func applyMetadataPatch(meta *Metadata, patch MetadataPatch) bool {
	changed := false
	if patch.Title != nil {
		meta.Titles = []DCMeta{{Value: *patch.Title}}
		changed = true
	}
	if patch.Language != nil {
		meta.Languages = []DCMeta{{Value: *patch.Language}}
		changed = true
	}
	if patch.Identifier != nil {
		if len(meta.Identifiers) == 0 {
			meta.Identifiers = []DCMeta{{Value: *patch.Identifier}}
		} else {
			meta.Identifiers[0].Value = *patch.Identifier
		}
		changed = true
	}
	if patch.Description != nil {
		meta.Descriptions = []DCMeta{{Value: *patch.Description}}
		changed = true
	}
	if patch.Creators != nil {
		meta.Creators = make([]DCMeta, 0, len(*patch.Creators))
		for _, name := range *patch.Creators {
			meta.Creators = append(meta.Creators, DCMeta{Value: name})
		}
		changed = true
	}
	return changed
}

func updateModifiedTimestamp(meta *Metadata) {
	stamp := time.Now().UTC().Format(time.RFC3339)
	for i := range meta.Meta {
		if meta.Meta[i].Property == "dcterms:modified" {
			meta.Meta[i].Value = stamp
			return
		}
	}
	meta.Meta = append(meta.Meta, MetaNode{
		Property: "dcterms:modified",
		Value:    stamp,
	})
}
