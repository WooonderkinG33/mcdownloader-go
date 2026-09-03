// smoke: полный прогон Ensure на лёгкой версии + замер скорости.
package main

import (
	"fmt"
	"os"
	"time"

	mcdownloader "github.com/WooonderkinG33/mcdownloader-go"
)

func main() {
	version := os.Getenv("MC_VERSION")
	if version == "" {
		version = "1.8.9"
	}
	dir := os.Getenv("MC_DIR")
	if dir == "" {
		dir = "/tmp/mc-downloader-smoke"
	}
	opts := mcdownloader.Options{
		MCVersion:        version,
		Modloader:        mcdownloader.Modloader(os.Getenv("MC_LOADER")),
		ModloaderVersion: os.Getenv("MC_MODLOADER_VERSION"),
		VersionDir:       dir,
		Concurrency:      8,
	}
	if os.Getenv("MC_JRE") == "1" {
		opts.DownloadJRE = true
		opts.JREDir = dir + "-jre"
	}
	d, err := mcdownloader.New(opts)
	if err != nil {
		fmt.Println("new:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	rep, err := d.Ensure()
	if err != nil {
		fmt.Println("ensure:", err)
		os.Exit(1)
	}
	fmt.Printf("SMOKE OK version=%s modloader=%s/%s main=%s libs=%d java>=%d javapath=%s total=%s\n",
		rep.MCVersion, rep.Modloader, rep.ModloaderVersion, rep.MainClass,
		len(rep.Libraries), rep.RequiredJavaMajor, rep.JavaPath, time.Since(t0).Round(time.Second))
}
