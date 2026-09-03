package mcdownloader

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// httpClient — один на либу, с таймаутами.
func newHTTPClient(timeoutSec int) *http.Client {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
}

// httpGetBytes — GET с 3 ретраями (серверные 5xx и сетевые ошибки).
func httpGetBytes(url string) ([]byte, error) {
	return httpGetBytesCli(newHTTPClient(30), url)
}

func httpGetBytesCli(cli *http.Client, url string) ([]byte, error) {
	var last error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := cli.Get(url)
		if err != nil {
			last = fmt.Errorf("attempt %d: %w", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
		resp.Body.Close()
		if err != nil {
			last = fmt.Errorf("attempt %d read: %w", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if resp.StatusCode >= 500 {
			last = fmt.Errorf("attempt %d: http %d", attempt, resp.StatusCode)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("http %d for %s", resp.StatusCode, url)
		}
		return body, nil
	}
	return nil, last
}

// sha1File считает sha1 файла.
func sha1File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fileOK — файл есть и (если хеш задан) совпадает.
func fileOK(path, sha1hex string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || st.Size() == 0 {
		return false
	}
	if sha1hex == "" {
		return true
	}
	got, err := sha1File(path)
	if err != nil {
		return false
	}
	return got == sha1hex
}

// downloadFile качает один файл с ретраями, пропускает если хеш сошёлся.
func downloadFile(cli *http.Client, f File) error {
	if fileOK(f.Path, f.SHA1) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	var last error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := downloadOnce(cli, f); err != nil {
			last = err
			if isPermanentHTTP(err) {
				return err
			}
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if f.SHA1 != "" {
			got, err := sha1File(f.Path)
			if err != nil || got != f.SHA1 {
				_ = os.Remove(f.Path)
				last = fmt.Errorf("sha1 mismatch for %s (want %s got %s)", f.Path, f.SHA1, got)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
		}
		return nil
	}
	return last
}

func downloadOnce(cli *http.Client, f File) error {
	tmp := f.Path + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	resp, err := cli.Get(f.URL)
	if err != nil {
		out.Close()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		out.Close()
		return &httpErr{code: resp.StatusCode, url: f.URL}
	}
	_, err = io.Copy(out, resp.Body)
	cerr := out.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if cerr != nil {
		os.Remove(tmp)
		return cerr
	}
	return os.Rename(tmp, f.Path)
}

type httpErr struct {
	code int
	url  string
}

func (e *httpErr) Error() string { return fmt.Sprintf("http %d for %s", e.code, e.url) }

// isPermanentHTTP — 4xx кроме 429 не ретраим.
func isPermanentHTTP(err error) bool {
	if he, ok := err.(*httpErr); ok {
		return he.code >= 400 && he.code < 500 && he.code != 429
	}
	return false
}

// batch качает файлы воркерами. Прогресс — в prog РЕАЛТАЙМ, на каждый файл.
// Троттлинг делает потребитель. skip404=true — 404 пропускать с варном
// (старые удалённые ассеты Mojang), иначе 404 фатален.
func batch(cli *http.Client, files []File, workers int, sub string, skip404 bool, prog func(p Progress)) error {
	if workers <= 0 {
		workers = 8
	}
	var total int64
	for _, f := range files {
		total += f.Size
	}
	var done atomic.Int64
	var completed atomic.Int64
	var skippedN atomic.Int64
	n := int64(len(files))

	report := func() {
		c := completed.Load()
		pct := 0
		if n > 0 {
			pct = int(c * 100 / n)
		}
		if c == n {
			pct = 100
		}
		if prog != nil {
			prog(Progress{Phase: "download", Sub: sub, Done: done.Load(), Total: total,
				Pct: pct, Text: fmt.Sprintf("(%d/%d файлов)", c, n)})
		}
	}

	jobs := make(chan File, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	var firstErr atomic.Value
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				if firstErr.Load() != nil {
					completed.Add(1)
					continue
				}
				if !fileOK(f.Path, f.SHA1) {
					if err := downloadFile(cli, f); err != nil {
						if skip404 {
							if he, ok := err.(*httpErr); ok && he.code == 404 {
								log.Printf("[download:%s] WARN skip 404 %s", sub, f.URL)
								skippedN.Add(1)
								completed.Add(1)
								report()
								continue
							}
						}
						if firstErr.Load() == nil {
							firstErr.Store(err)
						}
						completed.Add(1)
						report()
						continue
					}
				}
				done.Add(f.Size)
				completed.Add(1)
				report()
			}
		}()
	}
	wg.Wait()
	report()
	if v := firstErr.Load(); v != nil {
		return v.(error)
	}
	if skippedN.Load() > 0 {
		log.Printf("[download:%s] пропущено 404: %d", sub, skippedN.Load())
	}
	return nil
}
