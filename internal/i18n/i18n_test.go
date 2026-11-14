package i18n

import (
	"strings"
	"testing"
)

func TestWebTextsHaveBothLanguages(t *testing.T) {
	for k := range web.Zh {
		if _, ok := web.En[k]; !ok {
			t.Errorf("web.json: %s has no English text", k)
		}
	}
	for k := range web.En {
		if _, ok := web.Zh[k]; !ok {
			t.Errorf("web.json: %s has no Chinese text", k)
		}
	}
}

func TestNativeTextsHaveBothLanguages(t *testing.T) {
	for k, v := range native {
		if strings.TrimSpace(v[0]) == "" || strings.TrimSpace(v[1]) == "" {
			t.Errorf("%s is missing a language", k)
		}
		if strings.Count(v[0], "%") != strings.Count(v[1], "%") {
			t.Errorf("%s: zh and en have different format verbs", k)
		}
	}
}

func TestFallbacks(t *testing.T) {
	defer Set("zh")
	Set("en")
	if T("stop") != "Stop" || T("app_title") != "AI Passport App Library" {
		t.Fatal("English lookup failed")
	}
	if T("no-such-key") != "no-such-key" {
		t.Fatal("unknown keys should return the key")
	}
	if ErrText("busy", "x") == "x" || ErrText("nope", "raw") != "raw" {
		t.Fatal("ErrText fallback is wrong")
	}
	if Pick("", "en_US") != "en" || Pick("", "zh_CN") != "zh" || Pick("zh", "en") != "zh" {
		t.Fatal("Pick is wrong")
	}
}
