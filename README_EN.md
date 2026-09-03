# mcdownloader-go

**[Русский](README.md) | English**

---

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/doc/go1.22)
[![License: MIT](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Minecraft](https://img.shields.io/badge/Minecraft-1.5.2%20–%2026.2-555?style=for-the-badge)](https://www.minecraft.net)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows%20%7C%20macOS-555?style=for-the-badge)](#cross-platform)
[![Minecraft Client Downloader](https://img.shields.io/badge/Minecraft%20Client%20Downloader-Go-blue?style=for-the-badge)](.)

**keywords:** `minecraft` · `launcher` · `download` · `fabric` · `quilt` · `forge` · `piston-meta` · `mojang` · `jre` · `java` · `client` · `library` · `golang` · `go` · `minecraft client` · `minecraft launcher`

---

## What is it

**mcdownloader-go** is a Go library that turns a Minecraft version number into a fully launch-ready folder with a client.

It downloads `client.jar`, libraries, assets, natives, verifies integrity (`SHA1`), detects a compatible Java and lays everything flat into the folder you specify. **No UI, no CLI** — a clean Go API only.

Supports **vanilla**, **Fabric** and **Quilt** out of the box. **Forge / NeoForge** are planned (v0.3).

---

## Install

```bash
go get github.com/WooonderkinG33/mcdownloader-go@v1.0.0
```

---

## Quick start

**3 lines — client ready to launch:**

```go
package main

import (
    "fmt"
    mc "github.com/WooonderkinG33/mcdownloader-go"
)

func main() {
    d, _ := mc.New(mc.Options{
        MCVersion:  "1.20.1",
        VersionDir: "~/.minecraft/versions/1.20.1",
    })
    rep, _ := d.Ensure()
    fmt.Println(rep.ClientJar, rep.MainClass)
}
```

**With Fabric:**

```go
d, _ := mc.New(mc.Options{
    MCVersion:        "1.20.1",
    Modloader:        mc.Fabric,
    ModloaderVersion: "0.16.14",
    VersionDir:       "~/.minecraft/versions/1.20.1-fabric",
})
```

**With Java download:**

```go
d, _ := mc.New(mc.Options{
    MCVersion:   "1.20.1",
    VersionDir:  "~/.minecraft/versions/1.20.1",
    DownloadJRE: true,
    JREDir:      "~/.minecraft/runtime",
})
rep, _ := d.Ensure()
// rep.JavaPath = ~/.minecraft/runtime/java-runtime-gamma/bin/java
```

---

## Features

- **Vanilla** — from 1.5.2 to the latest (26.2+), via `piston-meta.mojang.com`
- **Fabric** — via `meta.fabricmc.net/v2`, explicit loader version required
- **Quilt** — via `meta.quiltmc.org/v3`, explicit loader version required
- **Forge / NeoForge** — not yet implemented (v0.3), return an error
- **Mojang JRE** — downloads a compatible runtime (`java-runtime-gamma`, etc.), path in `InstallReport`
- **SHA1 verification** — of every file during download + final re-check
- **Missing assets** — Mojang sometimes deletes old CDN files; the library skips 404 assets with a warning
- **Loader rotation** — switching loader versions removes old files, the client is not re-downloaded (3 seconds)
- **Realtime progress** — event per downloaded file + phase change
- **Cross-platform** — Linux / Windows / macOS: path separators, natives per architecture, system Java paths

---

## Constructor Options

### Required

| Field | Type | Description |
|-------|------|-------------|
| `MCVersion` | `string` | Minecraft version, e.g. `"1.20.1"`, `"1.8.9"`, `"26.2"` |
| `VersionDir` | `string` | Absolute path where the client is laid out. **Does not contain the version** — it's your folder: `"~/.minecraft/versions/1.20.1"` or `"~/projects/CraftopiaMC"` |

### Modloader

| Field | Type | Description |
|-------|------|-------------|
| `Modloader` | `Modloader` | `"vanilla"` / `mc.Fabric` / `mc.Quilt`. Empty = vanilla |
| `ModloaderVersion` | `string` | **Required when modloader is set.** No auto-latest — fresh loader + old MC = conflicts |

> **Why no auto-latest?** A fresh Fabric/Quilt on old Minecraft (e.g. 1.5.2) gives incompatibility. The user must specify the version deliberately.

### Java

| Field | Type | Description |
|-------|------|-------------|
| `DownloadJRE` | `bool` | Download Mojang runtime from `piston-meta`. Default `false` |
| `JREDir` | `string` | Where to put the runtime. **Required when `DownloadJRE: true`** |

### Performance & logging

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Concurrency` | `int` | `runtime.NumCPU() * 2` (4–16) | Number of parallel downloads |
| `HTTPTimeout` | `int` | `120` sec | Timeout per HTTP request |
| `Quiet` | `bool` | `false` | Disable stdout logger. With `Quiet: true` and no `Progress` — silence |
| `Progress` | `func(Progress)` | `nil` → `LogProgress` | Progress callback. `nil` → 10% logger. Provided → realtime without throttling |

---

## InstallReport

What `d.Ensure()` returns — what's downloaded and ready:

```go
type InstallReport struct {
    MCVersion         string   // "1.20.1"
    Modloader         Modloader // mc.Fabric / mc.Quilt / mc.Vanilla
    ModloaderVersion  string   // "0.16.14"
    ClientJar         string   // Absolute path to the .jar
    Libraries         []string // All .jar for classpath
    NativesDir        string   // Extracted .so/.dll/.dylib
    AssetsDir         string   // Assets folder
    AssetsID          string   // Asset index id ("5", "1.8")
    MainClass         string   // Entry point (Minecraft / KnotClient)
    RequiredJavaMajor int      // Required Java version (8, 17, 21, 25)
    JavaPath          string   // Path to JRE (only with DownloadJRE: true)
    Minecraft         string   // = VersionDir (gameDir)
}
```

---

## Progress

The library emits **realtime** events per downloaded file:

```go
type Progress struct {
    Phase string  // "init" | "resolve" | "download" | "verify" | "done"
    Sub   string  // "client+libs" | "assets" | "java"
    Done  int64   // Downloaded files
    Total int64   // Total files
    Pct   int     // 0–100
    Text  string  // Human-readable description
}
```

**Examples:**

```go
// 1. Default logger (every event, stdout)
Progress: mc.LogProgress

// 2. Silent (nothing to stdout)
Progress: func(p mc.Progress) {}

// 3. Rate-limited UI update (once per second)
var last time.Time
Progress: func(p mc.Progress) {
    if time.Since(last) < time.Second && p.Pct != 100 { return }
    last = time.Now()
    updateProgressUI(p.Pct, p.Text)
}

// 4. Into a state map (for getter tactics)
Progress: func(p mc.Progress) {
    statusMap[p.Phase] = p.Pct
    eventBus.Emit("download:progress", p)
}
```

---

## File layout

After `Ensure()` in `VersionDir`:

```
VersionDir/
├── 1.20.1.jar              # Client
├── natives/                # .so / .dll / .dylib
├── loader.json             # Loader version marker (fabric/quilt)
├── libraries/              # All .jar libraries
├── assets/
│   ├── indexes/
│   │   └── 5.json          # Asset index
│   └── objects/
│       ├── a1/...          # Assets by first 2 hash chars
│       └── ...
└── runtime/                # Only with DownloadJRE: true
    ├── java-runtime-gamma/
    │   ├── bin/java
    │   └── jre.json        # Marker (component, path)
    └── ...
```

---

## How to use

**Vanilla 1.8.9:**

```go
d, _ := mc.New(mc.Options{
    MCVersion:  "1.8.9",
    VersionDir: "/data/minecraft/1.8.9",
})
rep, _ := d.Ensure()
// rep.MainClass = "net.minecraft.client.main.Main"
// rep.RequiredJavaMajor = 8
```

**Fabric 1.20.1:**

```go
d, _ := mc.New(mc.Options{
    MCVersion:        "1.20.1",
    Modloader:        mc.Fabric,
    ModloaderVersion: "0.16.14",
    VersionDir:       "/data/minecraft/1.20.1-fabric",
})
rep, _ := d.Ensure()
// rep.MainClass = "net.fabricmc.loader.impl.launch.knot.KnotClient"
```

**With Mojang JRE:**

```go
d, _ := mc.New(mc.Options{
    MCVersion:   "1.20.1",
    VersionDir:  "/data/minecraft/1.20.1",
    DownloadJRE: true,
    JREDir:      "/data/minecraft/runtime",
})
rep, _ := d.Ensure()
// rep.JavaPath = "/data/minecraft/runtime/java-runtime-gamma/bin/java"
```

**Launch after Ensure:**

```go
rep, _ := d.Ensure()
classpath := rep.ClientJar + string(os.PathListSeparator) + strings.Join(rep.Libraries, string(os.PathListSeparator))
cmd := exec.Command(rep.JavaPath, "-Xmx4G", "-Djava.library.path="+rep.NativesDir,
    "-cp", classpath, rep.MainClass, "--username", "Player", "--version", rep.MCVersion,
    "--gameDir", rep.Minecraft, "--assetsDir", rep.AssetsDir, "--assetIndex", rep.AssetsID)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Run()
```

---

## Tested

### Vanilla

| Version | Status | Time | Size | Note |
|---------|--------|------|------|------|
| 1.5.2 | ✅ OK | 10s | 56 MB | `launchwrapper.Launch` — legacy format |
| 1.8.9 | ✅ OK | 17s | 136 MB | Menu loaded, LWJGL 2.9.4 |
| 1.12.2 | ✅ OK | 24s | 178 MB | Menu loaded |
| 1.16.5 | ✅ OK | — | — | Menu loaded, LWJGL 3.3.1 |
| 1.20.1 | ✅ OK | 80s | 706 MB | Menu loaded, Java 17 |
| 26.2 (latest) | ✅ OK | 80s | 584 MB | Menu loaded, Java 25 |

### Fabric

| MC Version | Fabric Version | Status | Note |
|------------|----------------|--------|------|
| 1.14.4 | 0.16.14 | ✅ OK | First supported version |
| 1.16.5 | 0.16.14 | ✅ OK | |
| 1.20.1 | 0.16.14 | ✅ OK | 60 libs |
| 26.2 | 0.19.5 | ✅ OK | 76 libs |

### Quilt

| MC Version | Quilt Version | Status | Note |
|------------|---------------|--------|------|
| 1.16.5 | 0.24.0 | ✅ OK | |
| 1.20.1 | 0.23.0 | ✅ OK | 67 libs |
| 26.2 | 0.24.0 | ❌ FAIL | `ASM` in quilt does not support Java 25 (class major 69) |

### Java Runtime

| Runtime | Status | Version | Size |
|---------|--------|---------|------|
| Mojang Gamma | ✅ OK | 17.0.15 (Microsoft build) | 96 MB |
| Mojang Epsilon | ✅ OK | 25.0.1 | 111 MB |

### Additional

- **Loader version switch** (fabric 0.16.14 → 0.15.11 and back): 3 seconds, no client re-download
- **404 assets**: Mojang deletes old CDN files — the library skips with a warning (like the official launcher)
- **go vet**: clean

---

## Cross-platform

| OS | Paths | Classpath | Natives | System Java |
|----|-------|-----------|---------|-------------|
| Linux | `~/.minecraft/...` | `:` separator | `.so` | `/usr/lib/jvm/...` |
| Windows | `%APPDATA%\.minecraft\...` | `;` separator | `.dll` | Registry, `JAVA_HOME` |
| macOS | `~/Library/Application Support/minecraft/...` | `:` separator | `.dylib` | `/Library/Java/JavaVirtualMachines/...` |

---

## Architecture

```
mcdownloader-go/
├── types.go       # Options, Modloader, Progress, File, InstallReport
├── core.go        # New(), Ensure(), LogProgress — entry point
├── init.go        # resolveDir, ensureDirs — environment, system Java
├── resolver.go    # piston-meta, Mojang types, rules, maven paths
├── loader.go      # Fabric/Quilt meta API, launcherMeta
├── downloader.go  # HTTP, retry, SHA1, batch workers
├── installer.go   # loader.json marker, natives extraction
├── jre.go         # Mojang JRE runtime
├── cmd/smoke/     # Test runner
└── example_*.go   # Integration examples
```

---

## Roadmap

| Version | What | Status |
|---------|------|--------|
| **v1.0** | Vanilla + Fabric + Quilt + Mojang JRE | ✅ Released |
| v0.3 | Forge + NeoForge (installer processors) | Planned |
| v0.3 | Unit tests (httptest) + CI | Planned |
| v0.3 | LegacyFabric (old versions 1.3–1.13) | On demand |

---

## License

MIT License — see [LICENSE](LICENSE).

---

## Acknowledgements

- [Mojang Studios](https://www.mojang.com/) — `piston-meta`, clients, Java runtimes
- [FabricMC](https://fabricmc.net/) — meta API, Maven repository
- [QuiltMC](https://quiltmc.org/) — meta API, Maven repository