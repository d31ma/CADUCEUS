import (
	"cmp"
	"slices"
	"strings"
)

// LocaleUtils is the part of shared/scripts/localization.js that settings
// needs: the fixed interface locale choices, their normalization, and the
// curated messages of the language section.
type LocaleUtils struct{}

func (utils LocaleUtils) Options(locale string) []LocaleOption {
	options := utils.choices()
	options[0].Label = utils.Translate(utils.automaticKey(), locale)
	return options
}

func (utils LocaleUtils) choices() []LocaleOption {
	return []LocaleOption{
		{ID: utils.automatic()},
		{ID: utils.english(), Label: "English"}, {ID: "de", Label: "Deutsch"}, {ID: "es", Label: "Español"},
		{ID: "es-419", Label: "Español (Latinoamérica)"}, {ID: "fr", Label: "Français"},
		{ID: "ko", Label: "한국어"}, {ID: "pl", Label: "Polski"}, {ID: "ja", Label: "日本語"},
		{ID: "zh-CN", Label: "简体中文"}, {ID: "zh-TW", Label: "繁體中文"}, {ID: "pt", Label: "Português"},
		{ID: "pt-BR", Label: "Português (Brasil)"}, {ID: "gl", Label: "Galego"}, {ID: "tr", Label: "Türkçe"},
		{ID: "ru", Label: "Русский"}, {ID: "cs", Label: "Čeština"}, {ID: "uk", Label: "Українська"},
		{ID: "hu", Label: "Magyar"}, {ID: "he", Label: "עברית"}, {ID: "ar", Label: "العربية"},
	}
}

func (utils LocaleUtils) Normalize(value any) string {
	text, _ := value.(string)
	return PreferenceUtils{}.Choice(text, utils.ids(), utils.automatic())
}

// Resolve picks the locale that a choice shows: the choice itself, or for
// "auto" the browser language when an interface locale matches it.
func (utils LocaleUtils) Resolve(choice string, language string) string {
	requested := FlowUtils{}.Text(choice == utils.automatic(), language, choice)
	return cmp.Or(utils.exact(requested), utils.primary(requested), utils.english())
}

// Translate answers a curated message, in English when the locale has none.
func (utils LocaleUtils) Translate(key string, locale string) string {
	columns := utils.columns()
	row := utils.row(key)
	return cmp.Or(row[columns[locale]], row[1], key)
}

func (LocaleUtils) automatic() string { return "auto" }

func (LocaleUtils) english() string { return "en" }

func (LocaleUtils) automaticKey() string { return "locale.automatic" }

func (utils LocaleUtils) ids() []string {
	ids := []string{}
	for _, option := range utils.choices() {
		ids = append(ids, option.ID)
	}
	return ids
}

func (utils LocaleUtils) exact(requested string) string {
	return utils.at(slices.IndexFunc(utils.ids()[1:], func(id string) bool { return strings.EqualFold(id, requested) }))
}

func (utils LocaleUtils) primary(requested string) string {
	separator := "-"
	language := strings.ToLower(strings.Split(requested, separator)[0])
	return utils.at(slices.IndexFunc(utils.ids()[1:], func(id string) bool { return strings.ToLower(strings.Split(id, separator)[0]) == language }))
}

func (utils LocaleUtils) at(index int) string {
	if index < 0 {
		return ""
	}
	return utils.ids()[1:][index]
}

// columns places each curated locale at its column in the message rows.
func (utils LocaleUtils) columns() map[string]int {
	ids := utils.ids()
	return map[string]int{ids[1]: 1, ids[5]: 2, ids[20]: 3}
}

func (utils LocaleUtils) row(key string) []string {
	rows := utils.messages()
	index := slices.IndexFunc(rows, func(row []string) bool { return row[0] == key })
	return append(rows, []string{key})[cmp.Or(index+1, len(rows)+1)-1]
}

func (utils LocaleUtils) messages() [][]string {
	return [][]string{
		{utils.automaticKey(), "Automatic (device language)", "Automatique (langue de l’appareil)", "تلقائي (لغة الجهاز)"},
		{"locale.heading", "Language and region", "Langue et région", "اللغة والمنطقة"},
		{"locale.description", "Choose the language used by CADUCEUS. Dates and numbers follow the same locale.", "Choisissez la langue de CADUCEUS. Les dates et les nombres suivent la même langue.", "اختر لغة CADUCEUS. تتبع التواريخ والأرقام اللغة نفسها."},
		{"locale.label", "Interface language", "Langue de l’interface", "لغة الواجهة"},
		{"locale.saved", "Language preference saved.", "Préférence linguistique enregistrée.", "تم حفظ تفضيل اللغة."},
		{"locale.saveError", "Could not save language preference.", "Impossible d’enregistrer la langue.", "تعذر حفظ تفضيل اللغة."},
	}
}
