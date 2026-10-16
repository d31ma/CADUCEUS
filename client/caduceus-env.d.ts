declare global {
  type ResolvedInterfaceLocale = Exclude<InterfaceLocaleChoice, 'auto'>
  type InterfaceLocaleId = InterfaceLocaleChoice

  /** Browser-only payloads shared with their Go model twins. */
  interface GrammarRange { offset: number; length: number }
  interface AttachmentOffer { index: number; filename: string; contentType: string; size: number; inline: boolean }

  interface Window {
    launchQueue?: { setConsumer: (consumer: (launch: { targetURL?: string; files: FileSystemHandle[] }) => void) => void }
    _caduceus?: {
      auth: {
        token: string | null
        email: string | null
        role: string | null
        domains: string[]
        isLoggedIn: boolean
        name?: string | null
        displayName?: string
        mailboxNames?: Record<string, string>
        externalAccounts?: Array<Record<string, any>>
        externalFolders?: Record<string, Array<Record<string, string>>>
      }
      accentColor: string
      accentPresets: Record<string, { label: string; primary: string; container: string; onContainer: string }>
      interfaceFont: string
      composerFont: string
      interfaceLocale: ResolvedInterfaceLocale
      interfaceLocaleChoice: InterfaceLocaleChoice
      notificationSounds: Record<'new-mail' | 'hit-send' | 'send' | 'confirm', 'none' | 'chime' | 'bell' | 'pop' | 'pulse'>
      themePreferences: {
        activeTheme: 'automatic' | 'light' | 'dark' | 'darkside' | 'less-is-more' | 'taiga' | 'ubuntu'
        lightTheme: 'light' | 'less-is-more' | 'taiga' | 'ubuntu'
        darkTheme: 'dark' | 'darkside'
      }
      mailPreferences: ReturnType<typeof import('./shared/scripts/mail-preferences.js').normalizeMailPreferences>
      screenshotMode: boolean
      setAccent: (key: string) => void
      setScreenshotMode: (enabled: boolean) => void
      mailboxLabel: (domain: string) => string
      setMailboxNames: (names: Record<string, string>) => void
      setFontPreferences: (preferences: { interfaceFont?: unknown; composerFont?: unknown }) => { interfaceFont: string; composerFont: string }
      setInterfaceLocale: (locale: unknown) => { choice: InterfaceLocaleChoice; locale: ResolvedInterfaceLocale; direction: 'ltr' | 'rtl' }
      setNotificationSounds: (sounds: unknown) => Record<string, string>
      setThemePreferences: (preferences: unknown) => Record<string, string>
      setUnreadBadgePreference: (preferences: unknown) => { scope: 'all' | 'domain'; domain: string }
      setKeyboardShortcutOverrides: (overrides: unknown) => Promise<{ ok: boolean; error?: string; overrides?: Record<string, string>; bindings?: Record<string, string> }>
      setMailPreferences: (preferences: unknown) => ReturnType<typeof import('./shared/scripts/mail-preferences.js').normalizeMailPreferences>
      cachePreferences: (response: Response) => Promise<void>
      playSound: (event: 'new-mail' | 'hit-send' | 'send' | 'confirm', soundId?: 'none' | 'chime' | 'bell' | 'pop' | 'pulse') => Promise<boolean>
      apiFetch: (path: string, options?: RequestInit) => Promise<Response | null>
      apiFetchCached?: (path: string, options?: { cache?: 'reload' | 'cache-first' | 'network-first'; invalidatePrefixes?: string[] }) => Promise<Response | null>
      toast: (msg: string, duration?: number) => void
      toastAction: (msg: string, action: { label: string; onClick: () => void }, duration?: number) => void
      queueOfflineRequest: (request: {
        id?: string
        kind: 'mail-action' | 'draft' | 'send' | 'snooze' | 'calendar'
        path: string
        method: 'POST' | 'PUT' | 'DELETE'
        body?: Record<string, unknown> | null
        message?: string
        coalesceKey?: string
        metadata?: Record<string, unknown>
      }) => Promise<{ id: string; kind: string; createdAt: number }>
      performMailAction: (
        ids: string[],
        patch: Partial<{ folder: string; labels: { add?: string[]; remove?: string[] }; read: boolean; starred: boolean }>,
        message: string,
      ) => Promise<{
        actionId: string
        status: 'applied' | 'undone'
        kind: string
        count: number
        expiresAt: string
        emails: Array<{ id: string; domain: string; folder: string; labels?: string[]; systemFolder?: string | null; read: boolean; starred: boolean }>
      } | null>
      navigate: (target?: string) => void
      openEmail: (email: string | { id: string }) => void
      openEmailId: (id: string) => void
      reportSpam: (id: string) => Promise<void>
      markNotSpam: (id: string) => Promise<void>
      compose: (prefill?: Record<string, unknown>) => void
      consumeComposePrefill: () => Record<string, string>
      handleLogin: (data: { token: string; email: string; name?: string; role?: string; domains?: string[] }, returnTo?: string) => void
      initials: (addr: string) => string
      bytesLabel: (size: number) => string
      formatDate: (iso: string) => string
      translate: (id: string, values?: Record<string, unknown>) => string
    }
    _caduceusInitialised?: boolean
    _caduceusPreferencesReady?: boolean
    _caduceusColumnResizersBound?: boolean
    _caduceusNavInitialised?: boolean
    _caduceusInboxRefresh?: () => void
    _caduceusEmailDetailSelect?: EventListener
    _caduceusEmailDetailAction?: EventListener
    _caduceusInboxNav?: EventListener
    _caduceusInboxMailboxNames?: EventListener
    _caduceusInboxExternalMailboxes?: EventListener
    _caduceusInboxNewMail?: EventListener
    _caduceusInboxMailSelect?: EventListener
    _caduceusInboxMailBack?: EventListener
    _caduceusNotificationAction?: (event: MessageEvent) => void
    _caduceusPwaLaunchBound?: boolean
    _caduceusOfflineActionsBound?: boolean
    _caduceusFolderMailSelect?: EventListener
    _caduceusFolderMailBack?: EventListener
    _caduceusDraftInputHandler?: EventListener
    _caduceusDraftOfflineWriteHandler?: EventListener
    _caduceusComposeSelectionHandler?: EventListener
    _caduceusComposeToolbarPointerHandler?: EventListener
    _caduceusComposeRichKeyHandler?: EventListener
    _caduceusComposeImageSelectionHandler?: EventListener
    _caduceusComposeRichInputHandler?: EventListener
    _caduceusComposeEditorObserver?: MutationObserver
    _caduceusGrammarInputHandler?: EventListener
    _caduceusGrammarCompositionStart?: EventListener
    _caduceusGrammarCompositionEnd?: EventListener
    _caduceusTranslationTargetHandler?: EventListener
    _caduceusTranslationClickHandler?: EventListener
    _caduceusShareVisibility?: EventListener
    _caduceusShareInterval?: ReturnType<typeof setInterval>
    _caduceusDraftVisibilityHandler?: EventListener
    _caduceusRecipientInputHandler?: EventListener
    _caduceusRecipientFocusHandler?: EventListener
    _caduceusRecipientKeyHandler?: EventListener
    _caduceusRecipientPointerHandler?: EventListener
    _caduceusRecipientBlurHandler?: EventListener
    _caduceusTemplateKeyHandler?: EventListener
    _caduceusTemplateInputHandler?: EventListener
    _caduceusTemplateCache?: Array<{ id: string; name: string; subject?: string; text?: string; to?: string; cc?: string }>
    _caduceusSignaturePasteHandler?: EventListener
    _caduceusSignatureInputHandler?: EventListener
    _caduceusMailPreferenceDraftHandler?: EventListener
    _caduceusSignatureCache?: Array<{ id: string; domain: string; name: string; text: string; defaultFor: string[] }>
    __caduceusSignatureEditId?: string
    _caduceusCalendarViewModeGuard?: EventListener
    _caduceusCalendarShortcutHandlers?: Record<string, EventListener>
    _caduceusCalendarDragHandlers?: Record<string, EventListener>
    _caduceusCalendarFocusHandler?: EventListener
    _caduceusCalendarOfflineWriteHandler?: EventListener
    _caduceusContactsCache?: {
      owner: string
      contacts: Array<Record<string, any>>
      groups: Array<Record<string, any>>
	  addressBooks: Array<{ accountId: string; addressBookId: string; name: string; accountLabel: string; mailboxAddress: string }>
      loadSequence: number
    }
    _caduceusCache?: any
    _caduceusUnreadBadge?: import('./shared/scripts/unread-badge.js').UnreadBadgeController
    _caduceusShortcuts?: any
    _overflowMenuLoaded?: number
    _caduceusShowToast?: (msg: string | { msg: string; duration?: number; action?: { label: string; onClick: () => void } }, duration?: number) => void
    _caduceusKeyboard?: {
      init: () => Promise<void>
      ready: () => Promise<void>
      destroy: () => void
      applyOverrides: (value: unknown) => Promise<{ ok: boolean, error?: string, overrides?: Record<string, string>, bindings?: Record<string, string> }>
      validateOverrides: (value: unknown) => { ok: boolean, error?: string, overrides?: Record<string, string>, bindings?: Record<string, string> }
      normalizeBinding: (value: unknown) => string | null
      displayBinding: (binding: string) => string
      getCatalog: () => Array<Record<string, any>>
      getBindings: () => Record<string, string>
      getOverrides: () => Record<string, string>
      getGroups: () => Array<{ label: string, rows: Array<{ label: string; displayBinding: string }> }>
      getContext: () => string
    }
    CADUCEUS_CONFIG?: {
      apiUrl?: string
      brandLogo?: string
    }
    __CADUCEUS_DISABLE_SW?: boolean
    __CADUCEUS_OFFLINE_TRANSLATION_FACTORY?: (options: {
      sourceLanguage: string
      targetLanguage: string
      onProgress?: (loaded: number) => void
    }) => Promise<{ translate: (text: string) => Promise<string>; destroy?: () => void }>
    __caduceusDuvayReady: Promise<void>
    Tac?: {
      modules: Map<string, unknown>
      register: (name: string, module: unknown) => unknown
      load: (name: string) => Promise<unknown>
      navigate: (path: string) => void
      rerender: (id: string, detail?: unknown) => Promise<void>
      provide: (key: string, value: unknown) => void
    }
  }

  /** Base class for Tachyon components and page companions */
  class Tac {
    /** Reactive state fields (prefixed with $) */
    [key: string]: unknown

    /** Props passed from parent component or route */
    props: Record<string, unknown>

    /** Tachyon framework helpers */
    tac: {
      isBrowser: boolean
      isServer: boolean
      onMount: (fn: () => void) => void
      emit: (name: string, detail?: unknown) => boolean
      rerender: () => void
      inject: <T>(key: string, fallback?: T) => T | undefined
      provide: (key: string, value: unknown) => void
      env: <T>(key: string, fallback?: T) => T | undefined
      fetch: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>
      bindPersistentFields: (ctrl: unknown) => void
    }

    /** Emit a custom event from this component */
    emit(name: string, detail?: unknown): boolean
  }
}

export {}
