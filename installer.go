package mcdownloader

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// loaderMarker — versions/<mc>/loader.json: какой загрузчик активен и его файлы.
// При смене версии удаляем только файлы старого загрузчика, остальное (клиент,
// ассеты — тяжёлое) не трогаем и не перекачиваем.
type loaderMarker struct {
	Loader  string   `json:"loader"`
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

func syncLoaderMarker(verDir, libsRoot, loader, version string, files []File) error {
	markerPath := filepath.Join(verDir, "loader.json")
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}

	var old loaderMarker
	if raw, err := os.ReadFile(markerPath); err == nil {
		_ = json.Unmarshal(raw, &old)
	}
	if old.Loader == loader && old.Version == version {
		return nil // та же версия — ничего делать не надо
	}
	if old.Version != "" {
		// чистим файлы старого загрузчика, которых нет в новом плане
		keep := map[string]bool{}
		for _, p := range paths {
			keep[p] = true
		}
		removed := 0
		for _, p := range old.Files {
			if !keep[p] {
				if err := os.Remove(p); err == nil {
					removed++
					// чистим пустые родительские папки вверх до libraries/
					for dir := filepath.Dir(p); dir != libsRoot && strings.HasPrefix(dir, libsRoot); dir = filepath.Dir(dir) {
						if err := os.Remove(dir); err != nil {
							break // не пустая — стоп
						}
					}
				}
			}
		}
		log.Printf("[loader] смена %s %s → %s: удалено старых файлов: %d, докачаем новые",
			loader, old.Version, version, removed)
	} else {
		log.Printf("[loader] %s %s: первая установка", loader, version)
	}
	raw, _ := json.Marshal(loaderMarker{Loader: loader, Version: version, Files: paths})
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(markerPath, raw, 0o644)
}

// extractNatives распаковывает .so/.dll/.dylib из classifier-jar в nativesDir.
func extractNatives(jars []File, nativesDir string) error {
	if err := os.MkdirAll(nativesDir, 0o755); err != nil {
		return fmt.Errorf("mkdir natives: %w", err)
	}
	for _, j := range jars {
		if err := extractNativeJar(j.Path, nativesDir); err != nil {
			return fmt.Errorf("natives %s: %w", j.Path, err)
		}
	}
	return nil
}

func extractNativeJar(jarPath, dest string) error {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		if strings.HasPrefix(name, "META-INF/") {
			continue
		}
		base := filepath.Base(name)
		if !(strings.HasSuffix(base, ".so") || strings.HasSuffix(base, ".dll") ||
			strings.HasSuffix(base, ".dylib") || strings.HasSuffix(base, ".jnilib")) {
			continue
		}
		out, err := os.Create(filepath.Join(dest, base))
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
