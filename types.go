// Package downloader — загрузка Minecraft-клиента до рабочего состояния.
//
// Только кодовый API, без UI. Все ошибки — обычный error с текстом,
// маппинг в красивое делает интегратор.
package mcdownloader

// Modloader — загрузчик модов.
type Modloader string

const (
	Vanilla  Modloader = "vanilla"
	Fabric   Modloader = "fabric"
	Quilt    Modloader = "quilt"
	Forge    Modloader = "forge"
	NeoForge Modloader = "neoforge"
)

// Options — всё, что нужно конструктору.
type Options struct {
	MCVersion string // "1.8.9", "1.12.2", "1.20.1" — обязательно
	// VersionDir — папка ИМЕННО ЭТОЙ версии/пресета, с кастомным именем (обязательно):
	// "~/.minerouter/minecraft/projects/CraftopiaMC". "~"/env раскрываются,
	// относительный → abs через cwd в New(). Игра запускается отсюда.
	VersionDir string

	Modloader        Modloader // "" = vanilla
	ModloaderVersion string    // ОБЯЗАТЕЛЬНА если Modloader задан

	Concurrency int // воркеры; 0 = авто 2×CPU [4..16]
	HTTPTimeout int // таймаут запроса в сек; 0 = 120

	// DownloadJRE: либа сама качает совместимый Mojang runtime.
	// JREDir ОБЯЗАТЕЛЕН при включённом DownloadJRE (куда класть).
	// Путь отдаём в InstallReport.JavaPath. Выключено = Java ищешь сам.
	DownloadJRE bool
	JREDir      string

	// Progress вызывается в РЕАЛТАЙМЕ: каждый скачанный файл + смена фазы.
	// Хочешь тишину — не передавай (тогда stdout 10% через LogProgress).
	// Хочешь свой вывод — передай колбэк (режь как хочешь: 1/с, 5%, молча).
	// Поля: Phase init|resolve|download|verify|done, Sub client+libs|assets|java,
	// Pct 0-100, Done/Total (файлы), Text — человекочитаемое.
	Progress func(p Progress)
}

// Progress — снимок прогресса.
type Progress struct {
	Phase string // init|resolve|download|verify|java|done
	Sub   string // client|libs|assets|natives|java
	Done  int64
	Total int64
	Pct   int   // 0-100
	Text  string
}

// File — один файл плана загрузки.
type File struct {
	URL  string
	Path string // абсолютный путь назначения
	SHA1 string // "" = без проверки
	Size int64  // 0 = неизвестен
}

// InstallReport — итог Ensure: версия скачана, проверена, склеена.
// Запуск (classpath, JVM args, процесс, поиск Java) — дело потребителя.
type InstallReport struct {
	MCVersion         string
	Modloader         Modloader
	ModloaderVersion  string
	ClientJar         string
	Libraries         []string // jar для classpath
	NativesDir        string
	AssetsDir         string
	AssetsID          string
	MainClass         string
	RequiredJavaMajor int    // из манифеста (0 = не указан, бери 8 для <1.13)
	JavaPath          string // "" если DownloadJRE выключен — ищи сам
	Minecraft         string // VersionDir (gameDir)
}
