const activeThemes = new Set(['automatic', 'light', 'dark', 'darkside', 'less-is-more', 'taiga', 'ubuntu'])
const lightThemes = new Set(['light', 'less-is-more', 'taiga', 'ubuntu'])
const darkThemes = new Set(['dark', 'darkside'])

/** Materialize the catalog-backed automatic and per-mode theme preferences. */
export default function upgrade(doc) {
  const existing = doc.themePreferences && typeof doc.themePreferences === 'object'
    ? doc.themePreferences
    : {}
  return {
    ...doc,
    themePreferences: {
      activeTheme: activeThemes.has(existing.activeTheme) ? existing.activeTheme : 'automatic',
      lightTheme: lightThemes.has(existing.lightTheme) ? existing.lightTheme : 'light',
      darkTheme: darkThemes.has(existing.darkTheme) ? existing.darkTheme : 'dark',
    },
  }
}
