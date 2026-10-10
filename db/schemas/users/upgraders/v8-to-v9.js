/** Add standalone grammar checking preferences, disabled by default. */
export default function upgrade(doc) {
  return {
    ...doc,
    grammarCheckEnabled: doc.grammarCheckEnabled === true,
    grammarCheckLanguage: ['auto', 'en-US', 'en-GB'].includes(doc.grammarCheckLanguage)
      ? doc.grammarCheckLanguage
      : 'auto',
    grammarCheckDisabledRules: Array.isArray(doc.grammarCheckDisabledRules)
      ? doc.grammarCheckDisabledRules.filter(rule => typeof rule === 'string' && /^[A-Z0-9_-]{1,64}$/.test(rule)).slice(0, 100)
      : [],
  }
}
