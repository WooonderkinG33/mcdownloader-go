package mcdownloader

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Downloader — фасад: New(Options) → Ensure() → InstallReport.
// Только скачивание/проверка/сборка. Запуск, classpath, поиск Java — потребитель.
type Downloader struct {
	opts Options
	root string
}

var logState = struct {
	mu   sync.Mutex
	last string
}{}

// New валидирует опции, резолвит пути. Сеть не трогает.
func New(opts Options) (*Downloader, error) {
	if strings.TrimSpace(opts.MCVersion) == "" {
		return nil, fmt.Errorf("MCVersion is required")
	}
	if strings.TrimSpace(opts.VersionDir) == "" {
		return nil, fmt.Errorf("VersionDir is required (куда качать именно эту версию)")
	}
	if opts.Modloader == "" {
		opts.Modloader = Vanilla
	}
	switch opts.Modloader {
	case Vanilla:
	case Fabric, Quilt:
		// свежий загрузчик на старом майнкрафте = конфликты:
		// версия обязательна, latest не подставляем
		if strings.TrimSpace(opts.ModloaderVersion) == "" {
			return nil, fmt.Errorf("missing required arg: ModloaderVersion (modloader %q без версии запрещён)", opts.Modloader)
		}
	case Forge, NeoForge:
		return nil, fmt.Errorf("modloader %q not implemented yet (v0.3, installer format)", opts.Modloader)
	default:
		return nil, fmt.Errorf("unknown modloader %q", opts.Modloader)
	}
	if opts.DownloadJRE && strings.TrimSpace(opts.JREDir) == "" {
		return nil, fmt.Errorf("JREDir is required when DownloadJRE is on (куда класть рантайм)")
	}
	if opts.Concurrency <= 0 {
		// мелкие файлы (ассеты) любят воркеры, но Mojang режет за 429:
		// 2×CPU, коридор 4..16. Больше 16 смысла нет — упираемся в сеть,
		// не в CPU; 32/64 только увеличат 429 и память.
		n := runtime.NumCPU() * 2
		if n < 4 {
			n = 4
		}
		if n > 16 {
			n = 16
		}
		opts.Concurrency = n
	}
	root, err := resolveDir(opts.VersionDir)
	if err != nil {
		return nil, err
	}
	opts.VersionDir = root
	if opts.JREDir != "" {
		jreDir, err := resolveDir(opts.JREDir)
		if err != nil {
			return nil, err
		}
		opts.JREDir = jreDir
	}
	if opts.HTTPTimeout <= 0 {
		// целый запрос включая тело: client.jar 20-50МБ на медленной сети
		// дольше 30с — 120с с ретраями ×3 надёжнее, кастомизируется полем
		opts.HTTPTimeout = 120
	}
	return &Downloader{opts: opts, root: root}, nil
}

func (d *Downloader) emit(p Progress) {
	// свой callback — всегда реалтайм, без троттлинга
	if d.opts.Progress != nil {
		d.opts.Progress(p)
		return
	}
	// без callback — дефолтный stdout-логгер 10% (для CLI/тестов).
	// Тишина = просто передай пустой callback func(p Progress) {}.
	LogProgress(p)
}

// LogProgress — готовый колбэк-логгер в stdout, пишет КАЖДОЕ событие (реалтайм).
// Передай как Options.Progress чтобы ничего не писать самому.
// Хочешь 10%/1-сек/молча — напиши свой колбэк с троттлингом, либа шлёт всё.
func LogProgress(p Progress) {
	logState.mu.Lock()
	defer logState.mu.Unlock()
	msg := fmt.Sprintf("[%s:%s] %d%% %s", p.Phase, p.Sub, p.Pct, p.Text)
	if msg == logState.last {
		return
	}
	logState.last = msg
	log.Print(msg)
}

// Ensure — полный конвейер: init → resolve → download → verify → java.
// Возвращает готовый InstallReport.
func (d *Downloader) Ensure() (*InstallReport, error) {
	t0 := time.Now()
	o := d.opts

	d.emit(Progress{Phase: "init", Pct: 0, Text: "root=" + d.root + " os=" + runtime.GOOS})
	if err := ensureDirs(d.root); err != nil {
		return nil, err
	}

	// всё плоско в VersionDir: вызыватель сам называет папку
	// (версией "…/1.20.1" или пресетом "…/CraftopiaMC")
	base := d.root

	// resolve
	d.emit(Progress{Phase: "resolve", Pct: 0, Text: "piston-meta " + o.MCVersion})
	r, err := resolveVersion(o.MCVersion, base, base)
	if err != nil {
		return nil, err
	}
	d.emit(Progress{Phase: "resolve", Pct: 100,
		Text: fmt.Sprintf("client+libs=%d natives=%d assets=%d main=%s java=%d",
			1+len(r.libs), len(r.natives), len(r.assets), r.mainClass, r.javaMajor)})

	cli := newHTTPClient(o.HTTPTimeout)

	// modloader: fabric/quilt через meta API (профиль даёт mainClass + deps)
	if o.Modloader == Fabric || o.Modloader == Quilt {
		d.emit(Progress{Phase: "resolve", Sub: string(o.Modloader), Pct: 0, Text: "meta " + o.MCVersion})
		lfiles, lmain, lver, err := resolveLoader(o.MCVersion, o.Modloader, o.ModloaderVersion, d.root)
		if err != nil {
			return nil, err
		}
		if lmain != "" {
			r.mainClass = lmain
		}
		r.libs = append(r.libs, lfiles...)
		// смена версии загрузчика: чистим файлы старого (маркер), всю сборку НЕ перекачиваем
		if err := syncLoaderMarker(base, filepath.Join(base, "libraries"), string(o.Modloader), lver, lfiles); err != nil {
			return nil, err
		}
		d.emit(Progress{Phase: "resolve", Sub: string(o.Modloader), Pct: 100,
			Text: fmt.Sprintf("modloader %s файлов=%d main=%s", lver, len(lfiles), r.mainClass)})
	}

	// download: client + libs (404 фатален)
	need := append([]File{r.client}, r.libs...)
	need = append(need, r.natives...)
	d.emit(Progress{Phase: "download", Sub: "client+libs", Pct: 0, Text: fmt.Sprintf("%d файлов", len(need))})
	if err := batch(cli, need, o.Concurrency, "client+libs", false, func(p Progress) {
		d.emit(p)
	}); err != nil {
		return nil, fmt.Errorf("download client+libs: %w", err)
	}

	// download: assets (404 пропускаем — старые удалённые объекты Mojang)
	d.emit(Progress{Phase: "download", Sub: "assets", Pct: 0, Text: fmt.Sprintf("%d объектов", len(r.assets))})
	if err := batch(cli, r.assets, o.Concurrency, "assets", true, func(p Progress) {
		d.emit(p)
	}); err != nil {
		return nil, fmt.Errorf("download assets: %w", err)
	}

	// verify: перепроверка хешей всего плана (молча по файлам, итог в лог)
	d.emit(Progress{Phase: "verify", Pct: 0, Text: "sha1 сверка"})
	bad := 0
	for _, f := range append(append([]File{r.client}, r.libs...), r.natives...) {
		if f.SHA1 != "" && !fileOK(f.Path, f.SHA1) {
			bad++
			log.Printf("[verify] MISMATCH %s", f.Path)
		}
	}
	if bad > 0 {
		return nil, fmt.Errorf("verify: %d файлов не сошлись по sha1", bad)
	}
	d.emit(Progress{Phase: "verify", Pct: 100, Text: "ok"})

	// natives extract — в папку версии
	nativesDir := filepath.Join(base, "natives")
	if err := extractNatives(r.natives, nativesDir); err != nil {
		return nil, err
	}

	// JRE: качаем Mojang runtime (компонент из манифеста версии), путь — в отчёт
	javaPath := ""
	if o.DownloadJRE {
		d.emit(Progress{Phase: "download", Sub: "java", Pct: 0, Text: "Mojang runtime"})
		jp, err := resolveJRE(r.detail.JavaVersion.Component, r.javaMajor, o.JREDir, func(p Progress) { d.emit(p) })
		if err != nil {
			return nil, err
		}
		javaPath = jp
	}

	// classpath
	var libs []string
	for _, f := range r.libs {
		libs = append(libs, f.Path)
	}
	rep := &InstallReport{
		MCVersion:         o.MCVersion,
		Modloader:         o.Modloader,
		ModloaderVersion:  o.ModloaderVersion,
		ClientJar:         r.client.Path,
		Libraries:         libs,
		NativesDir:        nativesDir,
		AssetsDir:         filepath.Join(d.root, "assets"),
		AssetsID:          r.assetsID,
		MainClass:         r.mainClass,
		RequiredJavaMajor: r.javaMajor,
		JavaPath:          javaPath,
		Minecraft:         d.root,
	}
	doneText := fmt.Sprintf("готово за %s: %s main=%s",
		time.Since(t0).Round(time.Second), r.client.Path, r.mainClass)
	if javaPath != "" {
		doneText += " java=" + javaPath
	} else {
		doneText += fmt.Sprintf(" java>=%d (ищи сам)", r.javaMajor)
	}
	d.emit(Progress{Phase: "done", Pct: 100, Text: doneText})
	return rep, nil
}
