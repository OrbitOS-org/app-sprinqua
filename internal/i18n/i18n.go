package i18n

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed locales/en.json
var enJSON []byte

//go:embed locales/pt.json
var ptJSON []byte

var locales map[string]map[string]string

func init() {
	locales = make(map[string]map[string]string)
	must(enJSON, "en")
	must(ptJSON, "pt")
}

func must(data []byte, lang string) {
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		panic("i18n: failed to parse " + lang + ".json: " + err.Error())
	}
	locales[lang] = m
}

// Strings returns the full translation map for the given language code.
// Falls back to "en" for unknown languages.
func Strings(lang string) map[string]string {
	if m, ok := locales[lang]; ok {
		return m
	}
	return locales["en"]
}

// Supported returns the list of supported language codes.
func Supported() []string {
	return []string{"en", "pt"}
}

// Detect picks a language from Accept-Language header, cookie, or falls back to "en".
// Priority: cookie > Accept-Language > "en".
func Detect(acceptLang, cookie string) string {
	if cookie == "pt" || cookie == "en" {
		return cookie
	}
	// Parse Accept-Language: e.g. "pt-PT,pt;q=0.9,en;q=0.8"
	for _, part := range strings.Split(acceptLang, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if strings.HasPrefix(strings.ToLower(tag), "pt") {
			return "pt"
		}
		if strings.HasPrefix(strings.ToLower(tag), "en") {
			return "en"
		}
	}
	return "en"
}
