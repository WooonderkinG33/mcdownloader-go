package mcdownloader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveDir раскрывает MinecraftDir в абсолютный путь:
// "" → дефолт по ОС, "~" → home, $VAR/%VAR% → env, относительный → abs через cwd.
func resolveDir(dir string) (string, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		switch runtime.GOOS {
		case "windows":
			appdata := os.Getenv("APPDATA")
			if appdata == "" {
				appdata = filepath.Join(home, "AppData", "Roaming")
			}
			return filepath.Join(appdata, ".minerouter", "minecraft"), nil
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "minerouter", "minecraft"), nil
		default:
			return filepath.Join(home, ".minerouter", "minecraft"), nil
		}
	}
	if strings.HasPrefix(dir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	dir = os.ExpandEnv(dir)
	if runtime.GOOS == "windows" {
		// %VAR% стиль
		for _, e := range os.Environ() {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) == 2 {
				dir = strings.ReplaceAll(dir, "%"+kv[0]+"%", kv[1])
			}
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}
	return filepath.Clean(abs), nil
}

// ensureDirs создаёт базу: libraries/, assets/.
// natives/ появляются сами при extract, пустые не плодим.
func ensureDirs(root string) error {
	for _, sub := range []string{"libraries", "assets"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}
	return nil
}


