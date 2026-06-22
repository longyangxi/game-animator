package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"perfectpixel/internal/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	xdraw "golang.org/x/image/draw"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
)

// GalleryImage is the metadata for a gallery/folder image file.
type GalleryImage struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"` // Unix milliseconds
}

var imageExtMime = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
}

const (
	maxImageFileBytes = 64 << 20 // upper limit for loading a single image
	maxFolderEntries  = 2000     // upper limit for folder listing
)

func ensureGalleryDir() (string, error) {
	dir, err := config.GalleryDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// GetGalleryPath ensures the gallery directory exists and returns its path.
func (a *App) GetGalleryPath() (string, error) {
	return ensureGalleryDir()
}

func listImagesIn(dir string) ([]GalleryImage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not read folder: %w", err)
	}
	items := make([]GalleryImage, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || imageExtMime[strings.ToLower(filepath.Ext(e.Name()))] == "" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, GalleryImage{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
		})
		if len(items) >= maxFolderEntries {
			break
		}
	}
	return items, nil
}

// ListGalleryImages returns gallery images sorted newest first.
func (a *App) ListGalleryImages() ([]GalleryImage, error) {
	dir, err := ensureGalleryDir()
	if err != nil {
		return nil, err
	}
	items, err := listImagesIn(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModTime > items[j].ModTime })
	return items, nil
}

// ListFolderImages returns images in the given folder sorted by name.
func (a *App) ListFolderImages(dir string) ([]GalleryImage, error) {
	items, err := listImagesIn(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// PickFolder opens a folder selection dialog and returns the chosen path (empty string if canceled).
func (a *App) PickFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select image folder"})
}

// DeleteGalleryImage deletes only images located inside the gallery directory.
func (a *App) DeleteGalleryImage(path string) error {
	dir, err := ensureGalleryDir()
	if err != nil {
		return err
	}
	clean := filepath.Clean(path)
	if filepath.Dir(clean) != dir {
		return errors.New("only images in the gallery folder can be deleted")
	}
	return os.Remove(clean)
}

func readImageFile(path string) ([]byte, string, error) {
	mime := imageExtMime[strings.ToLower(filepath.Ext(path))]
	if mime == "" {
		return nil, "", errors.New("unsupported image format")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("could not read file: %w", err)
	}
	if info.Size() > maxImageFileBytes {
		return nil, "", fmt.Errorf("image is too large (%.1fMB)", float64(info.Size())/(1<<20))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("could not read file: %w", err)
	}
	return data, mime, nil
}

// LoadImageFull returns the original image as a dataURL without re-encoding.
func (a *App) LoadImageFull(path string) (string, error) {
	data, mime, err := readImageFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// LoadImageThumb returns a downscaled thumbnail dataURL that fits within maxDim.
func (a *App) LoadImageThumb(path string, maxDim int) (string, error) {
	if maxDim <= 0 {
		maxDim = 200
	}
	data, mime, err := readImageFile(path)
	if err != nil {
		return "", err
	}
	orig := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	img, err := decodeImage(data)
	if err != nil {
		return orig, nil // on decode failure, defer to rendering the original
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return orig, nil
	}
	scale := float64(maxDim) / float64(max(w, h))
	nw := max(1, int(float64(w)*scale))
	nh := max(1, int(float64(h)*scale))
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return pngDataURL(dst)
}

var gallerySeq uint32

func galleryStamp() string {
	n := atomic.AddUint32(&gallerySeq, 1) % 1000
	return fmt.Sprintf("%s-%03d", time.Now().Format("20060102-150405"), n)
}

// saveGalleryPNG stores a single generated result in the gallery (the generation flow continues even on failure).
func saveGalleryPNG(name string, img image.Image) {
	dir, err := ensureGalleryDir()
	if err != nil {
		return
	}
	_ = writePNG(filepath.Join(dir, name+".png"), img)
}

// composeStrip joins frames horizontally into a single sprite strip.
func composeStrip(frames []*image.NRGBA) *image.NRGBA {
	if len(frames) == 0 {
		return nil
	}
	if len(frames) == 1 {
		return frames[0]
	}
	totalW, maxH := 0, 0
	for _, f := range frames {
		b := f.Bounds()
		totalW += b.Dx()
		if b.Dy() > maxH {
			maxH = b.Dy()
		}
	}
	strip := image.NewNRGBA(image.Rect(0, 0, totalW, maxH))
	x := 0
	for _, f := range frames {
		b := f.Bounds()
		xdraw.Copy(strip, image.Point{X: x, Y: 0}, f, b, xdraw.Over, nil)
		x += b.Dx()
	}
	return strip
}

// saveGalleryFrames stores a state's generated result in the gallery as a single horizontal sprite strip.
func saveGalleryFrames(state string, frames []*image.NRGBA) {
	strip := composeStrip(frames)
	if strip == nil {
		return
	}
	saveGalleryPNG(fmt.Sprintf("%s-%s", sanitizeName(state), galleryStamp()), strip)
}
