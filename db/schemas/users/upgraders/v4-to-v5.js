/** Materialize safe catalog-backed typography preference defaults. */
export default function upgrade(doc) {
  return {
    ...doc,
    interfaceFont: ['plex-sans', 'system', 'humanist', 'serif', 'monospace'].includes(doc.interfaceFont)
      ? doc.interfaceFont
      : 'plex-sans',
    composerFont: ['interface', 'plex-sans', 'system', 'humanist', 'serif', 'monospace'].includes(doc.composerFont)
      ? doc.composerFont
      : 'interface',
  }
}
