package mcdownloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// resolved — итог резолва: что качать и куда.
type resolved struct {
	detail    *versionDetail
	planURL   string
	client    File
	libs      []File
	natives   []File // classifier-jar под нашу ОС
	assets    []File
	assetsID  string
	mainClass string
	javaMajor int
}

// resolveVersion тянет piston-meta, находит версию, строит план файлов.
// root — корень (libraries/, assets/), verDir — папка версии
// (обычно сам root: всё плоско в переданной папке).
func resolveVersion(mcVersion, root, verDir string) (*resolved, error) {
	var manifest versionManifest
	if err := fetchJSON(pistonMetaURL, &manifest); err != nil {
		return nil, fmt.Errorf("version manifest: %w", err)
	}
	var detailURL string
	for _, v := range manifest.Versions {
		if v.ID == mcVersion {
			detailURL = v.URL
			break
		}
	}
	if detailURL == "" {
		return nil, fmt.Errorf("version %q not found in piston-meta", mcVersion)
	}
	var detail versionDetail
	if err := fetchJSON(detailURL, &detail); err != nil {
		return nil, fmt.Errorf("version detail: %w", err)
	}

	r := &resolved{
		detail:    &detail,
		planURL:   detailURL,
		assetsID:  detail.AssetIndex.ID,
		mainClass: detail.MainClass,
		javaMajor: detail.JavaVersion.MajorVersion,
	}

	// client.jar — в папку версии
	r.client = File{
		URL:  detail.Downloads.Client.URL,
		Path: filepath.Join(verDir, mcVersion+".jar"),
		SHA1: detail.Downloads.Client.SHA1,
		Size: detail.Downloads.Client.Size,
	}

	// libraries: artifact + natives под нашу ОС
	osName := mojangOSName()
	for _, lib := range detail.Libraries {
		if !ruleAllowed(lib.Rules) {
			continue
		}
		if lib.Downloads.Artifact != nil {
			a := lib.Downloads.Artifact
			url := a.URL
			if url == "" {
				url = librariesHost + "/" + a.Path
			}
			r.libs = append(r.libs, File{
				URL:  url,
				Path: filepath.Join(root, "libraries", filepath.FromSlash(a.Path)),
				SHA1: a.SHA1,
				Size: a.Size,
			})
		}
		// natives: classifier вида natives-linux / natives-windows / natives-osx
		if classifier, ok := lib.Natives[osName]; ok {
			classifier = strings.ReplaceAll(classifier, "${arch}", "64")
			g, a, v, _ := libCoord(lib.Name)
			nativeName := g + ":" + a + ":" + v + ":" + classifier
			nativePath := mavenPath(nativeName)
			r.natives = append(r.natives, File{
				URL:  librariesHost + "/" + nativePath,
				Path: filepath.Join(root, "libraries", filepath.FromSlash(nativePath)),
			})
		}
	}

	// assets: index + objects. Индекс сохраняем на диск —
	// игре нужен assets/indexes/<id>.json (без него ERROR в логе).
	var index assetObjects
	rawIndex, err := httpGetBytes(detail.AssetIndex.URL)
	if err != nil {
		return nil, fmt.Errorf("asset index: %w", err)
	}
	if err := json.Unmarshal(rawIndex, &index); err != nil {
		return nil, fmt.Errorf("asset index parse: %w", err)
	}
	indexPath := filepath.Join(root, "assets", "indexes", detail.AssetIndex.ID+".json")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir indexes: %w", err)
	}
	if err := os.WriteFile(indexPath, rawIndex, 0o644); err != nil {
		return nil, fmt.Errorf("write index: %w", err)
	}
	for _, o := range index.Objects {
		h := o.Hash
		sub := h[:2]
		r.assets = append(r.assets, File{
			URL:  resourcesHost + "/" + sub + "/" + h,
			Path: filepath.Join(root, "assets", "objects", sub, h),
			SHA1: h,
			Size: o.Size,
		})
	}

	return r, nil
}

// --- piston-meta: типы ответов Mojang, rules os/arch, maven-пути ---

const (
	pistonMetaURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
	resourcesHost = "https://resources.download.minecraft.net"
	librariesHost = "https://libraries.minecraft.net"
)

type versionManifest struct {
	Versions []versionEntry `json:"versions"`
}

type versionEntry struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type versionDetail struct {
	Downloads struct {
		Client struct {
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
		} `json:"client"`
	} `json:"downloads"`
	Libraries   []mojLibrary `json:"libraries"`
	AssetIndex  assetIndex   `json:"assetIndex"`
	MainClass   string       `json:"mainClass"`
	JavaVersion struct {
		Component    string `json:"component"`
		MajorVersion int    `json:"majorVersion"`
	} `json:"javaVersion"`
}

type mojLibrary struct {
	Name      string `json:"name"`
	Downloads struct {
		Artifact *struct {
			Path string `json:"path"`
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
		} `json:"artifact"`
	} `json:"downloads"`
	Natives map[string]string `json:"natives"`
	Rules   []mojRule        `json:"rules"`
}

type mojRule struct {
	Action string            `json:"action"`
	OS     map[string]string `json:"os"`
}

type assetIndex struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type assetObjects struct {
	Objects map[string]struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	} `json:"objects"`
}

// ruleAllowed проверяет rules библиотеки под текущие os/arch.
func ruleAllowed(rules []mojRule) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, r := range rules {
		match := true
		if osName, ok := r.OS["name"]; ok {
			match = mojangOSName() == osName
		}
		if arch, ok := r.OS["arch"]; ok && match {
			match = mojangArch(arch)
		}
		if r.Action == "allow" && match {
			allowed = true
		}
		if r.Action == "disallow" && match {
			return false
		}
	}
	return allowed
}

func mojangOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	default:
		return "linux"
	}
}

func mojangArch(arch string) bool {
	// rules Mojang: "x86" = 32-бит, остальное маппим на GOARCH
	goarch := runtime.GOARCH // amd64 | 386 | arm64
	switch arch {
	case "x86":
		return goarch == "386"
	case "x86_64", "amd64":
		return goarch == "amd64"
	case "arm64", "aarch64":
		return goarch == "arm64"
	default:
		return true // неизвестный arch — permissive
	}
}

// libCoord парсит maven-координаты group:artifact:version[:classifier].
func libCoord(name string) (group, artifact, version, classifier string) {
	p := strings.Split(name, ":")
	if len(p) > 0 {
		group = p[0]
	}
	if len(p) > 1 {
		artifact = p[1]
	}
	if len(p) > 2 {
		version = p[2]
	}
	if len(p) > 3 {
		classifier = p[3]
	}
	return
}

// mavenPath строит путь артефакта в libraries/.
func mavenPath(name string) string {
	g, a, v, c := libCoord(name)
	p := strings.ReplaceAll(g, ".", "/") + "/" + a + "/" + v + "/" + a + "-" + v
	if c != "" {
		p += "-" + c
	}
	p += ".jar"
	return p
}

func fetchJSON(url string, v any) error {
	body, err := httpGetBytes(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("parse %s: %w", url, err)
	}
	return nil
}
