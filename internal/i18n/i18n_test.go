package i18n

import "testing"

func TestSupportedLanguagesAndFallback(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{language: "en", want: "Sections"},
		{language: "ru", want: "Разделы"},
		{language: "es", want: "Secciones"},
		{language: "zh", want: "分区"},
		{language: "de", want: "Sections"},
	}
	for _, test := range tests {
		if got := New(test.language).Text("Sections"); got != test.want {
			t.Errorf("New(%q).Text() = %q, want %q", test.language, got, test.want)
		}
	}
}

func TestFormatAndPrefixTranslation(t *testing.T) {
	translator := New("ru")
	if got := translator.Format("Loaded %d items", 3); got != "Загружено элементов: 3" {
		t.Fatalf("Format() = %q", got)
	}
	if got := translator.Text("Invalid format: Policy"); got != "Некорректный формат: Policy" {
		t.Fatalf("prefix translation = %q", got)
	}
}
