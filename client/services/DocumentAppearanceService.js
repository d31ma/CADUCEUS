/**
 * Applies appearance choices to the document element: theme, interface
 * language, fonts, accent and the mail layout attributes that page styles
 * read. Only attributes change; the styles for them live in CSS. A
 * WebAssembly service cannot reach the document, so settings services
 * publish their choices and this service applies them.
 */
@Service
export default class DocumentAppearanceService {
  active = true
  /** @type {ThemePreferences | null} */
  _theme = null

  static _DARK = { dark: true, darkside: true }
  static _SCHEME = '(prefers-color-scheme: dark)'

  @OnStart
  start() {
    matchMedia(DocumentAppearanceService._SCHEME).addEventListener('change', () => this._applyTheme())
  }

  /** @param {ThemePreferences} preferences */
  @OnEvent
  theme(preferences) {
    this._theme = preferences
    this._applyTheme()
  }

  /** @param {LocalePreference} preference */
  @OnEvent
  locale(preference) {
    document.documentElement.setAttribute('data-interface-locale-choice', preference.choice)
  }

  /** @param {FontPreferences} preferences */
  @OnEvent
  fonts(preferences) {
    document.documentElement.setAttribute('data-interface-font', preferences.interfaceFont)
    document.documentElement.setAttribute('data-composer-font', preferences.composerFont)
  }

  /** @param {AccentColorChoice} choice */
  @OnEvent
  accent(choice) {
    document.documentElement.setAttribute('data-accent', choice.key)
  }

  /** @param {MailPreferences} preferences */
  @OnEvent
  mail(preferences) {
    const root = document.documentElement
    root.setAttribute('data-mail-layout', preferences.workspaceLayout)
    root.setAttribute('data-message-width', ['fluid', 'restricted'][Number(preferences.restrictMessageWidth)])
    root.setAttribute('data-message-theme', preferences.emailRenderMode)
    root.setAttribute('data-conversation-order', preferences.conversationOrder)
    root.setAttribute('data-attachment-previews', DocumentAppearanceService._enabled(preferences.displayAttachmentPreviews))
    root.setAttribute('data-rich-composer', DocumentAppearanceService._enabled(preferences.richText))
    root.setAttribute('data-mail-swipe', DocumentAppearanceService._enabled(preferences.swipeEnabled))
    root.setAttribute('data-interface-scale', String(preferences.interfaceScale))
  }

  _applyTheme() {
    const preferences = this._theme
    if (!preferences) return
    const resolved = DocumentAppearanceService._resolved(preferences, matchMedia(DocumentAppearanceService._SCHEME).matches)
    const mode = ['light', 'dark'][Number(Object.hasOwn(DocumentAppearanceService._DARK, resolved))]
    const root = document.documentElement
    root.setAttribute('w-theme', mode)
    root.setAttribute('data-theme', mode)
    root.setAttribute('data-ui-theme', resolved)
    root.setAttribute('data-theme-choice', preferences.activeTheme)
  }

  /** @param {ThemePreferences} preferences @param {boolean} dark */
  static _resolved(preferences, dark) {
    const automatic = [preferences.lightTheme, preferences.darkTheme][Number(dark)]
    return preferences.activeTheme === 'automatic' ? automatic : preferences.activeTheme
  }

  /** @param {boolean} enabled */
  static _enabled(enabled) {
    return ['disabled', 'enabled'][Number(enabled)]
  }
}
