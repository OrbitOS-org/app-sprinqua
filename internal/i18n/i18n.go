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

//go:embed locales/de.json
var deJSON []byte

//go:embed locales/es.json
var esJSON []byte

//go:embed locales/fr.json
var frJSON []byte

//go:embed locales/it.json
var itJSON []byte

var locales map[string]map[string]string

func init() {
	locales = make(map[string]map[string]string)
	must(enJSON, "en")
	must(ptJSON, "pt")
	must(deJSON, "de")
	must(esJSON, "es")
	must(frJSON, "fr")
	must(itJSON, "it")
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
	return []string{"en", "pt", "de", "es", "fr", "it"}
}

// Detect picks a language from Accept-Language header, cookie, or falls back to "en".
// Priority: cookie > Accept-Language > "en".
func Detect(acceptLang, cookie string) string {
	supported := Supported()
	for _, s := range supported {
		if cookie == s {
			return s
		}
	}
	for _, part := range strings.Split(acceptLang, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		for _, s := range supported {
			if strings.HasPrefix(tag, s) {
				return s
			}
		}
	}
	return "en"
}
