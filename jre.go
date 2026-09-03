package mcdownloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Рантайм Mojang — тот же JRE, что качает официальный лаунчер.
// Список: https://piston-meta.mojang.com/v1/products/java-runtime/<hash>/all.json
// Компонент (java-runtime-gamma и т.д.) берём из javaVersion манифеста версии.
const javaRuntimeListURL = "https://piston-meta.mojang.com/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json"

type jreList map[string]map[string][]struct {
	Version struct {
		Name     string `json:"name"`
		Released string `json:"released"`
	} `json:"version"`
	Manifest struct {
		URL string `json:"url"`
	} `json:"manifest"`
}

type jreManifest struct {
	Files map[string]struct {
		Type       string `json:"type"` // file | directory | link
		Target     string `json:"target"`
		Executable bool   `json:"executable"`
		Downloads  map[string]struct {
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
			URL  string `json:"url"`
		} `json:"downloads"`
	} `json:"files"`
}

func jreOSKey() string {
	switch runtime.GOOS {
	case "windows":
		return "windows-x86"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "mac-os-arm64"
		}
		return "mac-os"
	default:
		return "linux"
	}
}

// resolveJRE качает Mojang runtime (component из манифеста версии) в destDir/<component>/.
// Возвращает путь к бинарнику java. Повторный вызов — reuse по маркеру.
func resolveJRE(component string, required int, destDir string, prog func(p Progress)) (string, error) {
	if component == "" {
		// старые версии без javaVersion: legacy JRE 8
		component = "jre-legacy"
	}
	marker := filepath.Join(destDir, "jre.json")
	if raw, err := os.ReadFile(marker); err == nil {
		var m struct {
			Component string `json:"component"`
			Path      string `json:"path"`
		}
		if json.Unmarshal(raw, &m) == nil && m.Component == component {
			if st, err := os.Stat(m.Path); err == nil && !st.IsDir() {
				return m.Path, nil
			}
		}
	}

	var list jreList
	if err := fetchJSON(javaRuntimeListURL, &list); err != nil {
		return "", fmt.Errorf("jre list: %w", err)
	}
	osGroup, ok := list[jreOSKey()]
	if !ok {
		return "", fmt.Errorf("no jre for os %q", jreOSKey())
	}
	entries, ok := osGroup[component]
	if !ok || len(entries) == 0 {
		return "", fmt.Errorf("no jre component %q for %s", component, jreOSKey())
	}
	// свежий релиз первым
	best := entries[0]
	var bestTime time.Time
	for _, e := range entries {
		if rel, err := time.Parse(time.RFC3339, e.Version.Released); err == nil && rel.After(bestTime) {
			bestTime, best = rel, e
		}
	}

	var mf jreManifest
	if err := fetchJSON(best.Manifest.URL, &mf); err != nil {
		return "", fmt.Errorf("jre manifest: %w", err)
	}
	dest := filepath.Join(destDir, component)
	cli := newHTTPClient(60)
	type link struct{ path, target string }
	var links []link
	var files []File
	for rel, f := range mf.Files {
		p := filepath.Join(dest, filepath.FromSlash(rel))
		switch f.Type {
		case "directory":
			_ = os.MkdirAll(p, 0o755)
		case "link":
			links = append(links, link{p, f.Target})
		case "file":
			raw, ok := f.Downloads["raw"]
			if !ok {
				continue
			}
			files = append(files, File{URL: raw.URL, Path: p, SHA1: raw.SHA1, Size: raw.Size})
		}
	}
	n := int64(len(files))
	for i, f := range files {
		if err := downloadFile(cli, f); err != nil {
			return "", fmt.Errorf("jre file %s: %w", f.Path, err)
		}
		if prog != nil && (i+1 == int(n) || (i+1)*10/int(n) != i*10/int(n)) {
			prog(Progress{Phase: "download", Sub: "java", Done: int64(i + 1), Total: n,
				Pct: int((int64(i+1) * 100) / n), Text: fmt.Sprintf("JRE %s (%d/%d)", component, i+1, n)})
		}
	}
	// симлинки после файлов
	for _, l := range links {
		_ = os.MkdirAll(filepath.Dir(l.path), 0o755)
		_ = os.Remove(l.path)
		_ = os.Symlink(l.target, l.path)
	}
	bin := filepath.Join(dest, "bin", "java")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	} else {
		_ = os.Chmod(bin, 0o755)
	}
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("jre extracted but no bin/java in %s", dest)
	}
	raw, _ := json.Marshal(map[string]any{"component": component, "major": required, "path": bin})
	_ = os.WriteFile(marker, raw, 0o644)
	prog(Progress{Phase: "download", Sub: "java", Pct: 100, Text: "JRE " + bin})
	return bin, nil
}
