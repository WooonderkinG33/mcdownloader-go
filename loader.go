package mcdownloader

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Официальные meta API загрузчиков:
//
//	Fabric: https://meta.fabricmc.net/v2/versions/loader/{mc}/{loader}
//	Quilt:  https://meta.quiltmc.org/v3/versions/loader/{mc}/{loader}
//	Forge/NeoForge: инсталлерный формат, пока не поддерживаются (v0.3).
const (
	fabricMetaBase = "https://meta.fabricmc.net/v2"
	fabricMaven    = "https://maven.fabricmc.net/"
	quiltMetaBase  = "https://meta.quiltmc.org/v3"
	quiltMaven     = "https://maven.quiltmc.org/repository/release/"
)

// loaderProfile — общий формат ответа meta (fabric v2 / quilt v3).
type loaderProfile struct {
	Loader struct {
		Maven   string `json:"maven"`
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	} `json:"loader"`
	Intermediary struct {
		Maven   string `json:"maven"`
		Version string `json:"version"`
	} `json:"intermediary"`
	Hashed *struct {
		Maven string `json:"maven"`
	} `json:"hashed"`
	LauncherMeta struct {
		Libraries struct {
			Client []loaderLib `json:"client"`
			Common []loaderLib `json:"common"`
		} `json:"libraries"`
		MainClass struct {
			Client string `json:"client"`
		} `json:"mainClass"`
	} `json:"launcherMeta"`
}

type loaderLib struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// repoFor выбирает maven-репо по группе координат.
func repoFor(coord string) string {
	switch {
	case strings.HasPrefix(coord, "net.fabricmc:"):
		return fabricMaven
	case strings.HasPrefix(coord, "org.quiltmc:"):
		return quiltMaven
	default:
		return fabricMaven
	}
}

func loaderFile(coord, url, root string) File {
	base := url
	if base == "" {
		base = repoFor(coord)
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return File{
		URL:  base + mavenPath(coord),
		Path: filepath.Join(root, "libraries", filepath.FromSlash(mavenPath(coord))),
	}
}

// resolveLoader тянет профиль загрузчика, возвращает файлы в libraries/ и mainClass.
// modloaderVersion обязательна (latest не подставляем).
func resolveLoader(mc string, modloader Modloader, modloaderVersion, root string) (files []File, mainClass string, resolvedVersion string, err error) {
	var metaBase string
	switch modloader {
	case Fabric:
		metaBase = fabricMetaBase
	case Quilt:
		metaBase = quiltMetaBase
	default:
		return nil, "", "", fmt.Errorf("modloader %q not implemented yet (forge/neoforge — v0.3)", modloader)
	}

	var prof loaderProfile
	if err := fetchJSON(fmt.Sprintf("%s/versions/loader/%s/%s", metaBase, mc, modloaderVersion), &prof); err != nil {
		return nil, "", "", fmt.Errorf("modloader profile %s %s/%s: %w", modloader, mc, modloaderVersion, err)
	}
	if prof.Loader.Version == "" {
		return nil, "", "", fmt.Errorf("modloader %s/%s not found in meta", mc, modloaderVersion)
	}
	// loader jar + intermediary (+ hashed у quilt) — их нет в libraries.client
	for _, coord := range []string{prof.Loader.Maven, prof.Intermediary.Maven} {
		if coord != "" {
			files = append(files, loaderFile(coord, "", root))
		}
	}
	if prof.Hashed != nil && prof.Hashed.Maven != "" {
		files = append(files, loaderFile(prof.Hashed.Maven, "", root))
	}
	// deps: client + common (client у fabric пуст — всё в common)
	for _, lib := range append(prof.LauncherMeta.Libraries.Client, prof.LauncherMeta.Libraries.Common...) {
		files = append(files, loaderFile(lib.Name, lib.URL, root))
	}
	if modloader == Quilt {
		// quilt meta-профиль не содержит launchwrapper, а sponge-mixin
		// объявляет его сервис первым — без jar в classpath Knot падает
		// NoClassDefFoundError (официальный инсталлер добавляет его сам)
		const lw = "net.minecraft:launchwrapper:1.12"
		files = append(files, File{
			URL:  "https://libraries.minecraft.net/" + mavenPath(lw),
			Path: filepath.Join(root, "libraries", filepath.FromSlash(mavenPath(lw))),
		})
	}
	return files, prof.LauncherMeta.MainClass.Client, prof.Loader.Version, nil
}


