import type { GrammarRange } from './GrammarRange.ts'

export interface GrammarHighlightsChanged {
  ranges: GrammarRange[]
}
