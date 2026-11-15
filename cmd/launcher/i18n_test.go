package main

import "testing"

func TestLookupFallsBackToEnglishWhenKeyMissingInLocale(t *testing.T) {
	dicts := map[locale]map[string]string{
		localeEN:     {"greeting": "Hello"},
		localeZhHans: {}, // 这个语言缺该 key
	}
	if got := lookup(dicts, "greeting", localeZhHans); got != "Hello" {
		t.Fatalf("lookup() = %q, want English fallback %q", got, "Hello")
	}
}

func TestLookupReturnsKeyWhenMissingEverywhere(t *testing.T) {
	dicts := map[locale]map[string]string{localeEN: {}, localeZhHans: {}}
	if got := lookup(dicts, "totally.missing.key", localeEN); got != "totally.missing.key" {
		t.Fatalf("lookup() = %q, want the key itself as last-resort fallback", got)
	}
}

func TestLookupPrefersRequestedLocale(t *testing.T) {
	dicts := map[locale]map[string]string{
		localeEN:     {"greeting": "Hello"},
		localeZhHans: {"greeting": "你好"},
	}
	if got := lookup(dicts, "greeting", localeZhHans); got != "你好" {
		t.Fatalf("lookup() = %q, want %q", got, "你好")
	}
}

func TestToLocaleMapsChineseTagsToSimplified(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "zh-Hans-CN", "zh"} {
		if got := toLocale(tag); got != localeZhHans {
			t.Fatalf("toLocale(%q) = %q, want %q", tag, got, localeZhHans)
		}
	}
	for _, tag := range []string{"en", "en-US", "fr", ""} {
		if got := toLocale(tag); got != localeEN {
			t.Fatalf("toLocale(%q) = %q, want %q (English is the default)", tag, got, localeEN)
		}
	}
}

func TestEveryEnglishKeyHasChineseTranslation(t *testing.T) {
	for key := range messages[localeEN] {
		if _, ok := messages[localeZhHans][key]; !ok {
			t.Errorf("key %q has no zh-Hans translation", key)
		}
	}
	for key := range messages[localeZhHans] {
		if _, ok := messages[localeEN][key]; !ok {
			t.Errorf("key %q exists only in zh-Hans; English is the fallback and must have it", key)
		}
	}
}
