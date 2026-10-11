import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

// TranslationLanguageUtils reads the per-language translation choices that
// the message reader keeps on this device, under the local storage key
// caduceus-translation-preferences-v1, and names each language in English.
type TranslationLanguageUtils struct{}

// Stored reads the saved choices. A language that is never translated is
// not also translated automatically.
func (utils TranslationLanguageUtils) Stored(text string) TranslationChoices {
	stored := TranslationChoices{}
	json.Unmarshal([]byte(text), &stored)
	disabled := utils.primary(stored.Disabled)
	automatic := slices.DeleteFunc(utils.primary(stored.Automatic), func(language string) bool { return slices.Contains(disabled, language) })
	return TranslationChoices{Automatic: automatic, Disabled: disabled}
}

// Without forgets one language from both lists.
func (TranslationLanguageUtils) Without(choices TranslationChoices, language string) TranslationChoices {
	return TranslationChoices{Automatic: PreferenceUtils{}.Without(choices.Automatic, language), Disabled: PreferenceUtils{}.Without(choices.Disabled, language)}
}

// Encoded writes the choices the way the reader stores them.
func (TranslationLanguageUtils) Encoded(choices TranslationChoices) string {
	text, _ := json.Marshal(choices)
	return string(text)
}

// Rows names each language of a list.
func (utils TranslationLanguageUtils) Rows(languages []string) []LanguageOption {
	return utils.rows(languages, utils.labels())
}

func (TranslationLanguageUtils) rows(languages []string, labels map[string]string) []LanguageOption {
	rows := []LanguageOption{}
	for _, language := range languages {
		rows = append(rows, LanguageOption{ID: language, Label: cmp.Or(labels[language], language)})
	}
	return rows
}

// Key is the local storage key of the translation choices.
func (TranslationLanguageUtils) Key() string { return "caduceus-translation-preferences-v1" }

// primary keeps each language once, by its primary subtag in lower case.
func (TranslationLanguageUtils) primary(languages []string) []string {
	primary := []string{}
	for _, language := range languages {
		primary = append(primary, strings.Split(strings.ToLower(strings.TrimSpace(language)), "-")[0])
	}
	return PreferenceUtils{}.Unique(primary)
}

// labels are the English names of the reader's translation languages, from
// shared/scripts/translation-languages.js.
func (TranslationLanguageUtils) labels() map[string]string {
	return map[string]string{
		"af": "Afrikaans", "sq": "Albanian", "am": "Amharic", "ar": "Arabic", "hy": "Armenian", "az": "Azerbaijani",
		"ba": "Bashkir", "eu": "Basque", "be": "Belarusian", "bn": "Bengali", "bs": "Bosnian", "bg": "Bulgarian",
		"my": "Burmese", "ca": "Catalan", "ceb": "Cebuano", "zh": "Chinese", "hr": "Croatian", "cs": "Czech",
		"da": "Danish", "nl": "Dutch", "en": "English", "eo": "Esperanto", "et": "Estonian", "fi": "Finnish",
		"fr": "French", "gl": "Galician", "ka": "Georgian", "de": "German", "el": "Greek", "gu": "Gujarati",
		"ht": "Haitian Creole", "he": "Hebrew", "hi": "Hindi", "hu": "Hungarian", "mrj": "Hill Mari", "is": "Icelandic",
		"id": "Indonesian", "ga": "Irish", "it": "Italian", "ja": "Japanese", "jv": "Javanese", "kn": "Kannada",
		"kk": "Kazakh", "km": "Khmer", "ko": "Korean", "ky": "Kyrgyz", "lo": "Lao", "la": "Latin",
		"lv": "Latvian", "lt": "Lithuanian", "lb": "Luxembourgish", "mk": "Macedonian", "mg": "Malagasy", "ms": "Malay",
		"ml": "Malayalam", "mt": "Maltese", "mi": "Maori", "mhr": "Mari", "mr": "Marathi", "mn": "Mongolian",
		"ne": "Nepali", "no": "Norwegian", "pap": "Papiamento", "fa": "Persian", "pl": "Polish", "pt": "Portuguese",
		"pa": "Punjabi", "ro": "Romanian", "ru": "Russian", "gd": "Scottish Gaelic", "sr": "Serbian", "si": "Sinhala",
		"sk": "Slovak", "sl": "Slovenian", "es": "Spanish", "su": "Sundanese", "sw": "Swahili", "sv": "Swedish",
		"tg": "Tajik", "tl": "Tagalog", "ta": "Tamil", "tt": "Tatar", "te": "Telugu", "th": "Thai",
		"tr": "Turkish", "uk": "Ukrainian", "udm": "Udmurt", "ur": "Urdu", "uz": "Uzbek", "vi": "Vietnamese",
		"cy": "Welsh", "xh": "Xhosa", "yi": "Yiddish",
	}
}
