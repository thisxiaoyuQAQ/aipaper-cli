package i18n

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	tests := map[string]Language{
		"":            ZhCN,
		"zh-CN":       ZhCN,
		"zh":          ZhCN,
		"cn":          ZhCN,
		"chinese":     ZhCN,
		"中文":          ZhCN,
		"en":          En,
		"en-US":       En,
		"english":     En,
		"unsupported": ZhCN,
	}
	for input, want := range tests {
		if got := NormalizeLanguage(input); got != want {
			t.Fatalf("NormalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTranslatorFallbackAndFormat(t *testing.T) {
	tr := New("en")
	if got := tr.Text(WritingMetricsTitle); got != "📊 Metrics" {
		t.Fatalf("Text(en) = %q", got)
	}
	if got := New("fr").Text(WritingMetricsTitle); got != "📊 指标" {
		t.Fatalf("Text(fallback zh) = %q", got)
	}
	if got := tr.Text(Key("missing.key")); got != "missing.key" {
		t.Fatalf("missing key = %q", got)
	}
	if got := New("zh-CN").Format(SearchMaterialCandidates, 3); got != "材料候选：3" {
		t.Fatalf("Format() = %q", got)
	}
}

func TestIsSupported(t *testing.T) {
	if !IsSupported("english") || !IsSupported("") {
		t.Fatalf("expected supported aliases")
	}
	if IsSupported("fr") {
		t.Fatalf("fr should not be supported")
	}
}
