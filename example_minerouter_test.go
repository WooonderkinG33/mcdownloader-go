package mcdownloader_test

import (
	"fmt"
	"sync"
	"time"

	mcdownloader "github.com/WooonderkinG33/mcdownloader-go"
)

// Example_minerouter — реальная интеграция в MineRouter v2:
// либа шлёт реалтайм, мы режем до 1 обновления в секунду и маппим в lstate.
func Example_minerouter() {
	var mu sync.Mutex
	var lastEmit time.Time
	var lastPct int

	throttled := func(p mcdownloader.Progress) {
		mu.Lock()
		defer mu.Unlock()
		// 1 раз в секунду или финал — остальное выкидываем
		if time.Since(lastEmit) < time.Second && p.Pct != 100 {
			return
		}
		if p.Pct == lastPct && p.Phase == "download" {
			return
		}
		lastEmit, lastPct = time.Now(), p.Pct
		// у нас: lstate.Set(lstate.State{Phase: "download", Sub: p.Sub, Pct: p.Pct, Text: p.Text})
		fmt.Printf("lstate: %s/%s %d%% %s\n", p.Phase, p.Sub, p.Pct, p.Text)
	}

	d, _ := mcdownloader.New(mcdownloader.Options{
		MCVersion:        "1.20.1",
		Modloader:        mcdownloader.Fabric,
		ModloaderVersion: "0.16.14",
		VersionDir:       "~/.minerouter/minecraft/CraftopiaMC",
		DownloadJRE:      true, // рантайм Mojang в <VersionDir>/../runtime — нет:
		JREDir:           "~/.minerouter/minecraft/CraftopiaMC-runtime",
		Progress:         throttled,
	})
	_ = d
	// rep, _ := d.Ensure()
	// java := rep.JavaPath // готовый .../runtime/java-runtime-gamma/bin/java
	// cp := rep.ClientJar + ":" + strings.Join(rep.Libraries, ":")
	// exec(java, "-cp", cp, "-Djava.library.path="+rep.NativesDir, rep.MainClass, ...)
}
