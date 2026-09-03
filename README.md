# mcdownloader-go

**[English](README_EN.md) | Русский**

Библиотека на Go: указанная версия Minecraft → готовые файлы в указанной папке. Без UI, только кодовый API. `MIT`.

Берёт `Mojang piston-meta` (+ `Fabric`/`Quilt` meta), качает `client.jar`, `libraries`, `assets`, `natives`, проверяет `sha1`, складывает плоско в переданную папку. Запуск, `classpath`, поиск `Java` и процесс — дело потребителя.

```go
import mcdownloader "github.com/WooonderkinG33/mcdownloader-go"

d, _ := mcdownloader.New(mcdownloader.Options{
    MCVersion:  "1.20.1",
    VersionDir: "~/.minerouter/minecraft/CraftopiaMC",
})
rep, _ := d.Ensure() // 10 строк — и клиент скачан, проверен, склеен
```

## Возможности

- **Ванилла любых версий** — от `1.5.2` до свежих снапшотов, через официальный `piston-meta`
- **Fabric / Quilt** — через официальные meta API (`meta.fabricmc.net/v2`, `meta.quiltmc.org/v3`): loader jar, intermediary, deps, `mainClass` (`KnotClient`)
- **Mojang JRE** — опционально качает совместимый рантайм (`DownloadJRE`), путь отдаёт в отчёте
- **Проверки** — `sha1` каждого файла при скачивании + финальная сверка; `404` на протухших ассетах пропускаются с варном (как официальный лаунчер)
- **Ротация загрузчика** — `loader.json` маркер: смена версии чистит только файлы старого, сборка не перекачивается (3 секунды)
- **Прогресс реалтайм** — колбэк на каждый файл; троттлинг на потребителе
- **Кроссплатформа** — `linux/windows/macos` ветки путей, `classpath`-разделители, `natives` под ОС, `arch`-rules честно

## Установка

```sh
go get github.com/WooonderkinG33/mcdownloader-go
```

Требования: `Go 1.22+`, интернет к `piston-meta.mojang.com`, `resources.download.minecraft.net`, `libraries.minecraft.net` (+ `maven.fabricmc.net` / `maven.quiltmc.org` для модовых).

## Быстрый старт

```go
package main

import (
    "fmt"
    mcdownloader "github.com/WooonderkinG33/mcdownloader-go"
)

func main() {
    d, err := mcdownloader.New(mcdownloader.Options{
        MCVersion:  "1.20.1",
        VersionDir: "/tmp/my-mc", // назовётся как скажешь: версия или пресет
    })
    if err != nil {
        panic(err)
    }
    rep, err := d.Ensure()
    if err != nil {
        panic(err)
    }
    fmt.Println(rep.ClientJar, rep.MainClass) // готово к запуску
}
```

## Опции конструктора

### Обязательные

| Поле | Правило |
|---|---|
| `MCVersion` | всегда, например `"1.20.1"` |
| `VersionDir` | всегда — куда качать (`~`/env/относительный резолвятся в `New`) |
| `ModloaderVersion` | если `Modloader` задан (`latest` не подставляем осознанно — свежий лоадер на старый майн = конфликты) |
| `JREDir` | если `DownloadJRE: true` |

### Необязательные

| Поле | Дефолт | Описание |
|---|---|---|
| `Modloader` | `""` = vanilla | `Fabric` / `Quilt` (`Forge`/`NeoForge` — v0.3, вернут честную ошибку) |
| `Concurrency` | `0` = авто `2×CPU [4..16]` | воркеры; больше 16 бессмысленно (сеть + `429`) |
| `HTTPTimeout` | `0` = `120`с | таймаут запроса, ретраи ×3 |
| `DownloadJRE` | `false` | скачать Mojang runtime; выкл = `JavaPath == ""`, Java ищешь сам |
| `Progress` | `nil` = `LogProgress` в stdout | колбэк реалтайм (каждый файл + смена фазы) |

### Отчёт `InstallReport`

`ClientJar`, `Libraries[]`, `NativesDir`, `AssetsDir`/`AssetsID`, `MainClass`, `RequiredJavaMajor`, `JavaPath`, `Modloader`/`ModloaderVersion`, `Minecraft` (gameDir).

## Прогресс

Либа шлёт событие на каждый файл: `Phase` (`init/resolve/download/verify/done`), `Sub` (`client+libs/assets/java`), `Pct/Done/Total/Text`. Как показывать — решаешь ты:

```go
// молча
Progress: func(p mcdownloader.Progress) {},
// 10% в stdout
Progress: mcdownloader.LogProgress,
// не чаще раза в секунду (например, в lstate)
var last time.Time
Progress: func(p mcdownloader.Progress) {
    if time.Since(last) < time.Second && p.Pct != 100 {
        return
    }
    last = time.Now()
    updateUI(p)
},
```

## Реальная интеграция (запуск)

```go
rep, _ := d.Ensure()

java := rep.JavaPath
if java == "" {
    java = findJava(rep.RequiredJavaMajor) // свой поиск: PATH, /usr/lib/jvm, реестр
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

Полный пример с троттлингом — `example_minerouter_test.go`. Ручной прогон — `cmd/smoke` (`MC_VERSION`, `MC_LOADER`, `MC_MODLOADER_VERSION`, `MC_DIR`, `MC_JRE=1`).

## Раскладка результата

```
<VersionDir>/            ← имя задаёшь ты: "1.20.1" или "CraftopiaMC"
  1.20.1.jar             ← клиент
  natives/               ← распакованные .so/.dll
  loader.json            ← маркер активного модлоадера (модовые)
  libraries/             ← ванилла + загрузчик
  assets/objects/ + indexes/ ← ассеты + индекс
  runtime/               ← только при DownloadJRE (+ jre.json)
```

## Протестировано

Матрица 2026-09-03: скачивание + запуск на дисплее до главного меню.

| Версия | Vanilla | Fabric | Quilt |
|---|---|---|---|
| 1.5.2 | ✅ 10с / 56М | — (лоадера нет) | — |
| 1.8.9 | ✅ 17с / 136М | — (только LegacyFabric) | — |
| 1.12.2 | ✅ 24с / 178М | — (только LegacyFabric) | — |
| 1.14.4 | — | ✅ 0.16.14 (первая поддерживаемая) | — |
| 1.16.5 | ✅ | ✅ 0.16.14 | ✅ 0.24.0 |
| 1.20.1 | ✅ 80с / 706М | ✅ 0.16.14 | ✅ 0.23.0 |
| 26.2 | ✅ 80с / 584М | ✅ 0.19.5 | ❌ апстрим: ASM quilt 0.24.0 не читает классы Java 25 |

Дополнительно: `JRE gamma 17` + `epsilon 25` скачиваются и работают; ротация `0.16.14 ↔ 0.15.11` за 3с без перекачки; `go vet` чисто.

Известные ограничения: `Forge/NeoForge` (инсталлер с процессорами) — v0.3; `Windows`-ветки написаны, гонялись на `linux`.

## Дорожная карта

- v0.3: `Forge/NeoForge` (движок install-процессоров), юнит-тесты на `httptest`, `CI`
- По спросу: `LegacyFabric` (тот же meta-формат, +1 `case`), `Quilt` на новых MC по мере выхода

## Лицензия

`MIT` — см. `LICENSE`.
