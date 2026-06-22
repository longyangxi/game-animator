package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// Folder listing: verify only images are kept and they are sorted by name
func TestListFolderImages(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	if err := writePNG(filepath.Join(dir, "b.png"), img); err != nil {
		t.Fatal(err)
	}
	if err := writePNG(filepath.Join(dir, "a.png"), img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	items, err := app.ListFolderImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 images but got %d", len(items))
	}
	if items[0].Name != "a.png" || items[1].Name != "b.png" {
		t.Fatalf("sort by name failed: %s, %s", items[0].Name, items[1].Name)
	}
	if items[0].Size <= 0 || items[0].ModTime <= 0 {
		t.Fatalf("missing metadata: %+v", items[0])
	}
}

// Thumbnail: verify it is downscaled within maxDim while preserving aspect ratio
func TestLoadImageThumbDownscale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	if err := writePNG(path, image.NewNRGBA(image.Rect(0, 0, 600, 300))); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	url, err := app.LoadImageThumb(path, 200)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := decodeDataURL(url)
	if err != nil {
		t.Fatal(err)
	}
	thumb, err := decodeImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	b := thumb.Bounds()
	if b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("expected a 200x100 thumbnail but got %dx%d", b.Dx(), b.Dy())
	}
}

// A small image is returned as the original dataURL, with no re-encoding
func TestLoadImageThumbSmallPassthrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.png")
	if err := writePNG(path, image.NewNRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	url, err := app.LoadImageThumb(path, 200)
	if err != nil {
		t.Fatal(err)
	}
	full, err := app.LoadImageFull(path)
	if err != nil {
		t.Fatal(err)
	}
	if url != full {
		t.Fatal("a small image must be returned as the original dataURL unchanged")
	}
}

// Deleting a file outside the gallery directory must be rejected
func TestDeleteGalleryImageGuard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := writePNG(path, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	if err := app.DeleteGalleryImage(path); err == nil {
		t.Fatal("deleting a file outside the gallery must not be allowed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("file disappeared even though the delete request was rejected")
	}
}
