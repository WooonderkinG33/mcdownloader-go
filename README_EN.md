# mcdownloader-go

**[Русский](README.md) | English**

[![release](https://img.shields.io/github/v/release/WooonderkinG33/mcdownloader-go)](https://github.com/WooonderkinG33/mcdownloader-go/releases)
[![go](https://img.shields.io/badge/go-1.22+-blue)](https://go.dev)
[![license](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A Go library: given Minecraft version → ready files in a given folder. Code API only, no UI. `MIT`.

Takes `Mojang piston-meta` (+ `Fabric`/`Quilt` meta), downloads `client.jar`, `libraries`, `assets`, `natives`, verifies `sha1`, lays everything flat into the target folder. Launching, `classpath`, Java discovery and the process are the consumer's job.

```go
import mcdownloader "github.com/WooonderkinG33/mcdownloader-go"

d, _ := mcdownloader.New(mcdownloader.Options{
    MCVersion:  "1.20.1",
    VersionDir: "~/.minerouter/minecraft/CraftopiaMC",
})
rep, _ := d.Ensure() // 10 lines — client downloaded, verified, assembled
```

## Features

- **Vanilla of any version** — from `1.5.2` to latest snapshots via official `piston-meta`
- **Fabric / Quilt** — via official meta APIs (`meta.fabricmc.net/v2`, `meta.quiltmc.org/v3`): loader jar, intermediary, deps, `mainClass` (`KnotClient`)
- **Mojang JRE** — optionally downloads the matching runtime (`DownloadJRE`), path returned in the report
- **Verification** — `sha1` of every file on download + final re-check; `404`s on stale Mojang assets are skipped with a warning (like the official launcher)
- **Loader rotation** — `loader.json` marker: switching versions only removes old loader files, no full re-download (3 seconds)
- **Realtime progress** — callback per file; throttling is the consumer's job
- **Cross-platform** — `linux/windows/macos` path branches, `classpath` separators, per-OS `natives`, honest `arch` rules

## Install

```sh
go get github.com/WooonderkinG33/mcdownloader-go
```

Requires: `Go 1.22+`, internet access to `piston-meta.mojang.com`, `resources.download.minecraft.net`, `libraries.minecraft.net` (+ `maven.fabricmc.net` / `maven.quiltmc.org` for modded).

## Quick start

```go
package main

import (
    "fmt"
    mcdownloader "github.com/WooonderkinG33/mcdownloader-go"
)

func main() {
    d, err := mcdownloader.New(mcdownloader.Options{
        MCVersion:  "1.20.1",
        VersionDir: "/tmp/my-mc", // name it as you like: version or preset
    })
    if err != nil {
        panic(err)
    }
    rep, err := d.Ensure()
    if err != nil {
        panic(err)
    }
    fmt.Println(rep.ClientJar, rep.MainClass) // ready to launch
}
```

## Constructor options

### Required

| Field | Rule |
|---|---|
| `MCVersion` | always, e.g. `"1.20.1"` |
| `VersionDir` | always — where to download (`~`/env/relative resolved in `New`) |
| `ModloaderVersion` | when `Modloader` is set (no `latest` fallback by design — fresh loader on old MC = conflicts) |
| `JREDir` | when `DownloadJRE: true` |

### Optional

| Field | Default |
|---|---|
| `Modloader` | `""` = vanilla (`Fabric` / `Quilt`; `Forge`/`NeoForge` — v0.3, return an honest error) |
| `Concurrency` | `0` = auto `2×CPU [4..16]` (more than 16 is pointless — network + `429`s) |
| `HTTPTimeout` | `0` = `120`s, ×3 retries |
| `DownloadJRE` | `false` |
| `Progress` | `nil` = `LogProgress` to stdout |

### `InstallReport`

`ClientJar`, `Libraries[]`, `NativesDir`, `AssetsDir`/`AssetsID`, `MainClass`, `RequiredJavaMajor`, `JavaPath`, `Modloader`/`ModloaderVersion`, `Minecraft` (gameDir).

## Progress

The library emits an event per file: `Phase` (`init/resolve/download/verify/done`), `Sub` (`client+libs/assets/java`), `Pct/Done/Total/Text`. Display policy is yours:

```go
// silent
Progress: func(p mcdownloader.Progress) {},
// 10% to stdout
Progress: mcdownloader.LogProgress,
// at most once per second (e.g. into lstate)
var last time.Time
Progress: func(p mcdownloader.Progress) {
    if time.Since(last) < time.Second && p.Pct != 100 {
        return
    }
    last = time.Now()
    updateUI(p)
},
```

## Real integration (launch)

```go
rep, _ := d.Ensure()

java := rep.JavaPath
if java == "" {
    java = findJava(rep.RequiredJavaMajor) // your own search: PATH, /usr/lib/jvm, registry
}
cp := rep.ClientJar + string(os.PathListSeparator) + strings.Join(rep.Libraries, string(os.PathListSeparator))
cmd := exec.Command(java, append([]string{
    "-Xmx4G",
    "-Djava.library.path=" + rep.NativesDir,
    "-cp", cp,
    rep.MainClass,
    "--username", nick,
    "--version", rep.MCVersion,
    "--gameDir", rep.Minecraft,
    "--assetsDir", rep.AssetsDir,
    "--assetIndex", rep.AssetsID,
    "--uuid", uuid, "--accessToken", token,
    "--userProperties", "{}", "--userType", "legacy",
})...)
cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
_ = cmd.Run()
```

Full throttled example — `example_minerouter_test.go`. Manual runs — `cmd/smoke` (`MC_VERSION`, `MC_LOADER`, `MC_MODLOADER_VERSION`, `MC_DIR`, `MC_JRE=1`).

## Result layout

```
<VersionDir>/            ← you name it: "1.20.1" or "CraftopiaMC"
  1.20.1.jar             ← client
  natives/               ← extracted .so/.dll
  loader.json            ← active modloader marker (modded)
  libraries/             ← vanilla + loader
  assets/objects/ + indexes/ ← assets + index
  runtime/               ← only with DownloadJRE (+ jre.json)
```

## Tested

Matrix 2026-09-03: download + launch on display to main menu.

| Version | Vanilla | Fabric | Quilt |
|---|---|---|---|
| 1.5.2 | ✅ 10s / 56M | — (no loader exists) | — |
| 1.8.9 | ✅ 17s / 136M | — (LegacyFabric only) | — |
| 1.12.2 | ✅ 24s / 178M | — (LegacyFabric only) | — |
| 1.14.4 | — | ✅ 0.16.14 (first supported) | — |
| 1.16.5 | ✅ | ✅ 0.16.14 | ✅ 0.24.0 |
| 1.20.1 | ✅ 80s / 706M | ✅ 0.16.14 | ✅ 0.23.0 |
| 26.2 | ✅ 80s / 584M | ✅ 0.19.5 | ❌ upstream: ASM in quilt 0.24.0 can't read Java 25 classes |

Also: `JRE gamma 17` + `epsilon 25` download and run; rotation `0.16.14 ↔ 0.15.11` in 3s without re-download; `go vet` clean.

Known limits: `Forge/NeoForge` (installer with processors) — v0.3; `Windows` branches written, run on `linux`.

## Roadmap

- v0.3: `Forge/NeoForge` (install-processor engine), `httptest` unit tests, `CI`
- On demand: `LegacyFabric` (same meta shape, +1 `case`), `Quilt` on newer MC as released

## License

`MIT` — see `LICENSE`.
