package server

import (
	"embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed lang/*.json
var embeddedLangs embed.FS

var langCodePattern = regexp.MustCompile(`^[a-z]{2,5}$`)

// pageLangs holds the /qr and /pair page strings per language.
// English (built in) is the fallback for missing keys. Dropping a
// lang/<code>.json file next to the executable (or in the working
// directory's lang/) adds or overrides a language without recompiling.
type pageLangs struct {
	byCode map[string]map[string]string
	order  []string
}

func loadPageLangs() *pageLangs {
	pl := &pageLangs{byCode: map[string]map[string]string{}}

	add := func(code string, m map[string]string) {
		if _, ok := pl.byCode[code]; !ok {
			pl.order = append(pl.order, code)
		}
		pl.byCode[code] = m
	}

	// 1. Built-ins, English first so it always exists as fallback.
	for _, code := range []string{"en", "it"} {
		data, err := embeddedLangs.ReadFile("lang/" + code + ".json")
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			log.Printf("Builtin language %s is invalid: %v", code, err)
			continue
		}
		add(code, m)
	}

	// 2. External files override or extend the built-ins.
	for _, dir := range externalLangDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			files = append(files, e.Name())
		}
		sort.Strings(files)
		for _, name := range files {
			code := strings.TrimSuffix(name, ".json")
			if !langCodePattern.MatchString(code) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var m map[string]string
			if err := json.Unmarshal(data, &m); err != nil {
				log.Printf("Ignoring invalid language file %s: %v", name, err)
				continue
			}
			add(code, m)
		}
	}

	if _, ok := pl.byCode["en"]; !ok {
		pl.byCode["en"] = map[string]string{"name": "English"}
		pl.order = append([]string{"en"}, pl.order...)
	}
	return pl
}

// externalLangDirs returns lang/ directories that may hold extra
// translations: next to the executable first, then the working directory.
func externalLangDirs() []string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "lang"))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, "lang"))
	}
	return dirs
}

// has reports whether code is an available language.
func (pl *pageLangs) has(code string) bool {
	_, ok := pl.byCode[code]
	return ok
}

// codes returns available languages with English first.
func (pl *pageLangs) codes() []string {
	out := []string{"en"}
	for _, c := range pl.order {
		if c != "en" {
			out = append(out, c)
		}
	}
	return out
}

// name returns the language's native display name.
func (pl *pageLangs) name(code string) string {
	if m, ok := pl.byCode[code]; ok {
		if n := strings.TrimSpace(m["name"]); n != "" {
			return n
		}
	}
	return code
}

// get returns the string for code/key, falling back to English,
// then to the key itself.
func (pl *pageLangs) get(code, key string) string {
	if m, ok := pl.byCode[code]; ok {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
	}
	if m, ok := pl.byCode["en"]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}
