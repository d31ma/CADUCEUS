import (
	"encoding/json"
	"slices"
	"strings"
)

// LocaleUtils holds the curated interface catalogs (English, French and
// Arabic, generated from shared/locales/*.js as JSON text) and resolves the
// interface locale. It is the Go home of the catalog part of
// shared/scripts/localization.js; the generated literal catalogs for the
// other locales stay in shared/assets/interface-locales.json, which the
// page-wide interface localizer applies to rendered text.
type LocaleUtils struct{}

// Translate answers the message for id in locale, falling back to English
// and then to the id itself, and fills {name} placeholders from values.
func (LocaleUtils) Translate(locale string, id string, values map[string]string) string {
	return LocaleUtils{}.filled(LocaleUtils{}.pattern(LocaleUtils{}.catalog(locale)[id], LocaleUtils{}.catalog(LocaleUtils{}.english())[id], id), values)
}

// Resolve turns a stored choice ("auto" or a locale) and the browser language
// into a locale that has a curated catalog.
func (LocaleUtils) Resolve(choice string, language string) string {
	return LocaleUtils{}.supported(LocaleUtils{}.requested(choice, language))
}

// Direction is "rtl" for Arabic and Hebrew, "ltr" otherwise.
func (LocaleUtils) Direction(locale string) string {
	return LocaleUtils{}.direction(slices.Contains(LocaleUtils{}.list(`["ar","he"]`), locale))
}

// Choice keeps a known choice and turns anything else into "auto".
func (LocaleUtils) Choice(value string) string {
	return LocaleUtils{}.known(slices.Contains(LocaleUtils{}.choices(), value), value)
}

// Options are the interface language choices in their own script, with the
// automatic choice named in locale.
func (LocaleUtils) Options(locale string) []LocaleOption {
	options := []LocaleOption{}
	json.Unmarshal([]byte(LocaleUtils{}.optionLabels()), &options)
	options[0].Label = LocaleUtils{}.Translate(locale, "locale.automatic", map[string]string{})
	return options
}

func (LocaleUtils) requested(choice string, language string) string {
	if choice == "" || choice == (LocaleUtils{}).automatic() {
		return strings.ToLower(strings.SplitN(language, "-", 2)[0])
	}
	return choice
}

func (LocaleUtils) supported(locale string) string {
	if _, found := (LocaleUtils{}).catalogs()[locale]; found {
		return locale
	}
	return LocaleUtils{}.english()
}

func (LocaleUtils) direction(rightToLeft bool) string {
	if rightToLeft {
		return "rtl"
	}
	return "ltr"
}

func (LocaleUtils) known(found bool, value string) string {
	if found {
		return value
	}
	return LocaleUtils{}.automatic()
}

func (LocaleUtils) pattern(localized string, english string, id string) string {
	switch {
	case localized != "":
		return localized
	case english != "":
		return english
	}
	return id
}

func (LocaleUtils) filled(pattern string, values map[string]string) string {
	for name, value := range values {
		pattern = strings.ReplaceAll(pattern, "{"+name+"}", value)
	}
	return pattern
}

func (LocaleUtils) catalog(locale string) map[string]string {
	catalog := map[string]string{}
	json.Unmarshal([]byte(LocaleUtils{}.catalogs()[locale]), &catalog)
	return catalog
}

func (LocaleUtils) choices() []string {
	return LocaleUtils{}.list(`["auto","en","de","es","es-419","fr","ko","pl","ja","zh-CN","zh-TW","pt","pt-BR","gl","tr","ru","cs","uk","hu","he","ar"]`)
}

func (LocaleUtils) list(text string) []string {
	values := []string{}
	json.Unmarshal([]byte(text), &values)
	return values
}

// optionLabels names each entry of choices() in its own language, in the
// same order.
func (LocaleUtils) optionLabels() string {
	return `[{"id":"auto"},{"id":"en","label":"English"},{"id":"de","label":"Deutsch"},{"id":"es","label":"Español"},{"id":"es-419","label":"Español (Latinoamérica)"},{"id":"fr","label":"Français"},{"id":"ko","label":"한국어"},{"id":"pl","label":"Polski"},{"id":"ja","label":"日本語"},{"id":"zh-CN","label":"简体中文"},{"id":"zh-TW","label":"繁體中文"},{"id":"pt","label":"Português"},{"id":"pt-BR","label":"Português (Brasil)"},{"id":"gl","label":"Galego"},{"id":"tr","label":"Türkçe"},{"id":"ru","label":"Русский"},{"id":"cs","label":"Čeština"},{"id":"uk","label":"Українська"},{"id":"hu","label":"Magyar"},{"id":"he","label":"עברית"},{"id":"ar","label":"العربية"}]`
}

func (LocaleUtils) automatic() string { return "auto" }

func (LocaleUtils) english() string { return LocaleUtils{}.choices()[1] }

// catalogs are the curated messages of each locale, as JSON text.
func (LocaleUtils) catalogs() map[string]string {
	catalogs := map[string]json.RawMessage{}
	json.Unmarshal([]byte(LocaleUtils{}.curated()), &catalogs)
	texts := map[string]string{}
	for locale, catalog := range catalogs {
		texts[locale] = string(catalog)
	}
	return texts
}

// curated is the JSON text of the curated catalogs, keyed by locale.
func (LocaleUtils) curated() string {
	return `{"en":{"locale.automatic":"Automatic (device language)","locale.en":"English","locale.fr":"French","locale.ar":"Arabic","locale.heading":"Language and region","locale.description":"Choose the language used by CADUCEUS. Dates and numbers follow the same locale.","locale.label":"Interface language","locale.saved":"Language preference saved.","locale.saveError":"Could not save language preference.","shell.compose":"Compose","shell.notifications":"Notifications","shell.closeNotifications":"Close notifications","shell.noNotifications":"No active notifications.","shell.dismiss":"Dismiss","shell.online":"Online","shell.screenshotMode":"Screenshot mode","shell.exitScreenshotMode":"Exit screenshot mode","shell.settings":"Settings","shell.signOut":"Sign out","shell.favorites":"Favorites","shell.toggleInboxes":"Toggle inboxes","shell.toggleSent":"Toggle sent","shell.allInboxes":"All Inboxes","shell.allSent":"All Sent","shell.reminders":"Reminders","shell.activity":"Activity","shell.contacts":"Contacts","shell.calendar":"Calendar","shell.team":"Team","shell.inbox":"Inbox","shell.drafts":"Drafts","shell.sent":"Sent","shell.scheduled":"Scheduled","shell.snoozed":"Snoozed","shell.archive":"Archive","shell.spam":"Spam","shell.trash":"Trash","shell.folders":"Folders","shell.openNavigation":"Open navigation menu","shell.closeNavigation":"Close navigation menu","shell.resizeSidebar":"Resize sidebar","notification.count#one":"{count} notification","notification.count#other":"{count} notifications"},"fr":{"locale.automatic":"Automatique (langue de l’appareil)","locale.en":"Anglais","locale.fr":"Français","locale.ar":"Arabe","locale.heading":"Langue et région","locale.description":"Choisissez la langue de CADUCEUS. Les dates et les nombres suivent la même langue.","locale.label":"Langue de l’interface","locale.saved":"Préférence linguistique enregistrée.","locale.saveError":"Impossible d’enregistrer la langue.","shell.compose":"Écrire","shell.notifications":"Notifications","shell.closeNotifications":"Fermer les notifications","shell.noNotifications":"Aucune notification active.","shell.dismiss":"Ignorer","shell.online":"En ligne","shell.screenshotMode":"Mode capture d’écran","shell.exitScreenshotMode":"Quitter le mode capture d’écran","shell.settings":"Réglages","shell.signOut":"Se déconnecter","shell.favorites":"Favoris","shell.toggleInboxes":"Afficher ou masquer les boîtes de réception","shell.toggleSent":"Afficher ou masquer les messages envoyés","shell.allInboxes":"Toutes les boîtes","shell.allSent":"Tous les messages envoyés","shell.reminders":"Rappels","shell.activity":"Activité","shell.contacts":"Contacts","shell.calendar":"Calendrier","shell.team":"Équipe","shell.inbox":"Boîte de réception","shell.drafts":"Brouillons","shell.sent":"Envoyés","shell.scheduled":"Programmés","shell.snoozed":"Reportés","shell.archive":"Archives","shell.spam":"Indésirables","shell.trash":"Corbeille","shell.folders":"Dossiers","shell.openNavigation":"Ouvrir le menu de navigation","shell.closeNavigation":"Fermer le menu de navigation","shell.resizeSidebar":"Redimensionner la barre latérale","notification.count#one":"{count} notification","notification.count#other":"{count} notifications"},"ar":{"locale.automatic":"تلقائي (لغة الجهاز)","locale.en":"الإنجليزية","locale.fr":"الفرنسية","locale.ar":"العربية","locale.heading":"اللغة والمنطقة","locale.description":"اختر لغة CADUCEUS. تتبع التواريخ والأرقام اللغة نفسها.","locale.label":"لغة الواجهة","locale.saved":"تم حفظ تفضيل اللغة.","locale.saveError":"تعذر حفظ تفضيل اللغة.","shell.compose":"إنشاء رسالة","shell.notifications":"الإشعارات","shell.closeNotifications":"إغلاق الإشعارات","shell.noNotifications":"لا توجد إشعارات نشطة.","shell.dismiss":"تجاهل","shell.online":"متصل","shell.screenshotMode":"وضع لقطة الشاشة","shell.exitScreenshotMode":"إنهاء وضع لقطة الشاشة","shell.settings":"الإعدادات","shell.signOut":"تسجيل الخروج","shell.favorites":"المفضلة","shell.toggleInboxes":"إظهار أو إخفاء صناديق الوارد","shell.toggleSent":"إظهار أو إخفاء الرسائل المرسلة","shell.allInboxes":"كل صناديق الوارد","shell.allSent":"كل الرسائل المرسلة","shell.reminders":"التذكيرات","shell.activity":"النشاط","shell.contacts":"جهات الاتصال","shell.calendar":"التقويم","shell.team":"الفريق","shell.inbox":"الوارد","shell.drafts":"المسودات","shell.sent":"المرسلة","shell.scheduled":"المجدولة","shell.snoozed":"المؤجلة","shell.archive":"الأرشيف","shell.spam":"الرسائل غير المرغوب فيها","shell.trash":"المهملات","shell.folders":"المجلدات","shell.openNavigation":"فتح قائمة التنقل","shell.closeNavigation":"إغلاق قائمة التنقل","shell.resizeSidebar":"تغيير حجم الشريط الجانبي","notification.count#zero":"لا توجد إشعارات","notification.count#one":"إشعار واحد","notification.count#two":"إشعاران","notification.count#few":"{count} إشعارات","notification.count#many":"{count} إشعارًا","notification.count#other":"{count} إشعار"}}`
}
