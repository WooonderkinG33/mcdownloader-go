# mcdownloader-go

**[English](README_EN.md) | Русский**

---

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/doc/go1.22)
[![License: MIT](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Minecraft](https://img.shields.io/badge/Minecraft-1.5.2%20–%2026.2-555?style=for-the-badge&logo=)](https://www.minecraft.net)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows%20%7C%20macOS-555?style=for-the-badge)](#кроссплатформа)
[![Minecraft Client Downloader](https://img.shields.io/badge/Minecraft%20Client%20Downloader-Go-blue?style=for-the-badge)](.)

**keywords:** `minecraft` · `launcher` · `download` · `fabric` · `quilt` · `forge` · `piston-meta` · `mojang` · `jre` · `java` · `client` · `library` · `golang` · `go` · `качалка` · `загрузчик` · `майнкрафт`

---

## Что это

**mcdownloader-go** — Go-библиотека, которая превращает номер версии Minecraft в полностью готовую к запуску папку с клиентом.

Скачивает `client.jar`, библиотеки, ассеты, нативы, проверяет целостность (`SHA1`), определяет совместимую Java и раскладывает всё плоско в указанную папку. **Без UI, без CLI** — только чистый Go API.

Поддерживает **vanilla**, **Fabric** и **Quilt** из коробки. **Forge / NeoForge** — запланированы (v0.3).

---

## Установка

```bash
go get github.com/WooonderkinG33/mcdownloader-go@v1.0.0
```

---

## Быстрый старт

**3 строки — и клиент готов к запуску:**

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

**С Fabric:**

```go
d, _ := mc.New(mc.Options{
    MCVersion:        "1.20.1",
    Modloader:        mc.Fabric,
    ModloaderVersion: "0.16.14",
    VersionDir:       "~/.minecraft/versions/1.20.1-fabric",
})
```

**С загрузкой Java:**

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

## Возможности

- **Vanilla** — от 1.5.2 до последних версий (26.2+), через `piston-meta.mojang.com`
- **Fabric** — через `meta.fabricmc.net/v2`, нужна явная версия лоадера
- **Quilt** — через `meta.quiltmc.org/v3`, нужна явная версия лоадера
- **Forge / NeoForge** — пока не реализованы (v0.3), возвращают ошибку
- **Mojang JRE** — скачивает совместимый рантайм (`java-runtime-gamma` и др.), путь в `InstallReport`
- **SHA1-проверка** — каждого файла при скачивании + финальная сверка
- **Отсутствующие ассеты** — Mojang иногда удаляет старые файлы CDN, библиотека пропускает 404 ассетов с предупреждением
- **Ротация лоадера** — при смене версии лоадера старые файлы удаляются, клиент не перекачивается (3 секунды)
- **Прогресс реалтайм** — событие на каждый скачанный файл + смену фазы
- **Кроссплатформа** — Linux / Windows / macOS: разделители путей, natives под архитектуру, system Java paths

---

## Конструктор Options

### Обязательные параметры

| Поле | Тип | Описание |
|------|-----|----------|
| `MCVersion` | `string` | Версия Minecraft, например `"1.20.1"`, `"1.8.9"`, `"26.2"` |
| `VersionDir` | `string` | Абсолютный путь, куда раскладывается клиент. **Не содержит версию** — это твоя папка: `"~/.minecraft/versions/1.20.1"` или `"~/projects/CraftopiaMC"` |

### Модлоадер

| Поле | Тип | Описание |
|------|-----|----------|
| `Modloader` | `Modloader` | `"vanilla"` / `mc.Fabric` / `mc.Quilt`. Пустой = vanilla |
| `ModloaderVersion` | `string` | **Обязательна при модлоадере.** Нет авто-определения latest —Fresh loader + old MC = conflicts |

> **Почему нет auto-latest?** Свежий Fabric/Quilt на старом майнкрафте (например 1.5.2) даст несовместимость. Пользователь должен указать версию осознанно.

### Java

| Поле | Тип | Описание |
|------|-----|----------|
| `DownloadJRE` | `bool` | Скачать Mojang runtime из `piston-meta`. По умолчанию `false` |
| `JREDir` | `string` | Куда складывать runtime. **Обязательна при `DownloadJRE: true`** |

### Производительность и логирование

| Поле | Тип | По умолчанию | Описание |
|------|-----|-------------|----------|
| `Concurrency` | `int` | `runtime.NumCPU() * 2` (4–16) | Число параллельных загрузок |
| `HTTPTimeout` | `int` | `120` сек | Таймаут на один HTTP-запрос |
| `Quiet` | `bool` | `false` | Отключить stdout-логгер. При `Quiet: true` и без `Progress` — тишина |
| `Progress` | `func(Progress)` | `nil` → `LogProgress` | Колбэк для прогресса. При `nil` — логгер каждые 10%. При передаче — реалтайм без троттлинга |

---

## InstallReport

Отчёт `d.Ensure()` — что скачано и готово к использованию:

```go
type InstallReport struct {
    MCVersion         string   // "1.20.1"
    Modloader         Modloader // mc.Fabric / mc.Quilt / mc.Vanilla
    ModloaderVersion  string   // "0.16.14"
    ClientJar         string   // Абсолютный путь к .jar
    Libraries         []string // Все .jar для classpath
    NativesDir        string   // Распакованные .so/.dll/.dylib
    AssetsDir         string   // Папка assets
    AssetsID          string   // ID ассет-индекса ("5", "1.8")
    MainClass         string   // Точка входа (Minecraft / KnotClient)
    RequiredJavaMajor int      // Требуемая версия Java (8, 17, 21, 25)
    JavaPath          string   // Путь к JRE (только при DownloadJRE: true)
    Minecraft         string   // = VersionDir (gameDir)
}
```

---

## Прогресс

Библиотека шлёт события **реалтайм** на каждый скачанный файл:

```go
type Progress struct {
    Phase string  // "init" | "resolve" | "download" | "verify" | "done"
    Sub   string  // "client+libs" | "assets" | "java"
    Done  int64   // Скачано файлов
    Total int64   // Всего файлов
    Pct   int     // 0–100
    Text  string  // Человекочитаемое описание
}
```

**Примеры:**

```go
// 1. Дефолтный логгер (every event, stdout)
Progress: mc.LogProgress

// 2. Молча (ничего в stdout)
Progress: func(p mc.Progress) {}

// 3. Лимитированное обновление UI (1 раз в секунду)
var last time.Time
Progress: func(p mc.Progress) {
    if time.Since(last) < time.Second && p.Pct != 100 { return }
    last = time.Now()
    updateProgressUI(p.Pct, p.Text)
}

// 4. Через map (для getr tactics)
Progress: func(p mc.Progress) {
    statusMap[p.Phase] = p.Pct
    eventBus.Emit("download:progress", p)
}
```

---

## Раскладка файлов

После `Ensure()` в `VersionDir` лежит:

```
VersionDir/
├── 1.20.1.jar              # Клиент
├── natives/                # .so / .dll / .dylib
├── loader.json             # Маркер версии лоадера (fabric/quilt)
├── libraries/              # Все .jar библиотеки
├── assets/
│   ├── indexes/
│   │   └── 5.json          # Ассет-индекс
│   └── objects/
│       ├── a1/...          # Ассеты по первым 2 символам хеша
│       └── ...
└── runtime/                # Только при DownloadJRE: true
    ├── java-runtime-gamma/
    │   ├── bin/java
    │   └── jre.json        # Маркер (component, path)
    └── ...
```

---

## Как использовать

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

**С Mojang JRE:**

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

**Запуск после Ensure:**

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

## Тесты

### Vanilla

| Версия | Статус | Время | Размер | Примечание |
|--------|--------|-------|--------|------------|
| 1.5.2 | ✅ OK | 10с | 56 МБ | `launchwrapper.Launch` — старый формат |
| 1.8.9 | ✅ OK | 17с | 136 МБ | Меню загружено, LWJGL 2.9.4 |
| 1.12.2 | ✅ OK | 24с | 178 МБ | Меню загружено |
| 1.16.5 | ✅ OK | — | — | Меню загружено, LWJGL 3.3.1 |
| 1.20.1 | ✅ OK | 80с | 706 МБ | Меню загружено, Java 17 |
| 26.2 (latest) | ✅ OK | 80с | 584 МБ | Меню загружено, Java 25 |

### Fabric

| Версия MC | Fabric Версия | Статус | Примечание |
|-----------|--------------|--------|------------|
| 1.14.4 | 0.16.14 | ✅ OK | Первая поддерживаемая версия |
| 1.16.5 | 0.16.14 | ✅ OK | |
| 1.20.1 | 0.16.14 | ✅ OK | 60 либ |
| 26.2 | 0.19.5 | ✅ OK | 76 либ |

### Quilt

| Версия MC | Quilt Версия | Статус | Примечание |
|-----------|-------------|--------|------------|
| 1.16.5 | 0.24.0 | ✅ OK | |
| 1.20.1 | 0.23.0 | ✅ OK | 67 либ |
| 26.2 | 0.24.0 | ❌ FAIL | `ASM` в quilt не поддерживает Java 25 (class major 69) |

### Java Runtime

| Runtime | Статус | Версия | Размер |
|---------|--------|--------|--------|
| Mojang Gamma | ✅ OK | 17.0.15 (Microsoft build) | 96 МБ |
| Mojang Epsilon | ✅ OK | 25.0.1 | 111 МБ |

### Платформы

| ОС | Статус |
|----|--------|
| **Linux** | ✅ Протестирован: вся матрица версий и лоадеров |
| **Windows** | 🟡 Код + кросс-компиляция OK, живьём не гонялся |
| **macOS** | 🟡 Код + кросс-компиляция OK, живьём не гонялся |

### Известные ограничения

- **Смена версии лоадера** (fabric 0.16.14 → 0.15.11 и обратно): 3 секунды, без перекачки клиента
- **Ассеты с 404**: Mojang удаляет старые файлы CDN — библиотека пропускает с предупреждением (как оригинальный лаунчер)
- **go vet**: чисто

---

## Кроссплатформа

| ОС | Пути | Classpath | Natives | System Java |
|----|------|-----------|---------|-------------|
| Linux | `~/.minecraft/...` | `:` разделитель | `.so` | `/usr/lib/jvm/...` |
| Windows | `%APPDATA%\.minecraft\...` | `;` разделитель | `.dll` | Реестр, `JAVA_HOME` |
| macOS | `~/Library/Application Support/minecraft/...` | `:` разделитель | `.dylib` | `/Library/Java/JavaVirtualMachines/...` |

> **Статус платформ честно:** живьём протестирован **только Linux** (вся матрица выше). Ветки для **Windows** и **macOS** написаны (пути, natives, JRE `mac-os`/`mac-os-arm64`, разделители classpath) и **проходят кросс-компиляцию** (`GOOS=windows/amd64`, `GOOS=darwin/arm64` — OK), но на живом Windows/macOS **не гонялись** — ждём теста или пул-реквеста от пользователей этих ОС.

---

## Архитектура

```
mcdownloader-go/
├── types.go       # Options, Modloader, Progress, File, InstallReport
├── core.go        # New(), Ensure(), LogProgress — входная точка
├── init.go        # resolveDir, ensureDirs — окружение, системная Java
├── resolver.go    # piston-meta, Mojang типы, rules, maven-пути
├── loader.go      # Fabric/Quilt meta API, launcherMeta
├── downloader.go  # HTTP, retry, SHA1, batch-воркеры
├── installer.go   # loader.json маркер, распаковка natives
├── jre.go         # Mojang JRE runtime (Adoptium/Mojang)
├── cmd/smoke/     # Тестовый запуск
└── example_*.go   # Примеры интеграции
```

---

## Дорожная карта

| Версия | Что | Статус |
|--------|-----|--------|
| **v1.0** | Vanilla + Fabric + Quilt + Mojang JRE | ✅ Выпущена |
| v0.3 | Forge + NeoForge (инсталлер-процессоры) | Запланировано |
| v0.3 | Юнит-тесты (httptest) + CI | Запланировано |
| v0.3 | LegacyFabric (старые версии 1.3–1.13) | По спросу |

---

## Лицензия

MIT License —详见 [LICENSE](LICENSE).

---

## Благодарности

- [Mojang Studios](https://www.mojang.com/) — `piston-meta`, клиенты, рантаймы Java
- [FabricMC](https://fabricmc.net/) — meta API, Maven-репозиторий
- [QuiltMC](https://quiltmc.org/) — meta API, Maven-репозиторий
- [Cloudflare CIRCL](https://github.com/cloudflare/circl) — крипто-праймитивы (для будущего)
