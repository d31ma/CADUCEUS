import { describe, expect, it } from 'bun:test'
import { isForwardedSubject, searchConversation, splitQuotedText } from '../../client/shared/scripts/message-conversation.js'

describe('Mailspring-compatible conversation body behavior', () => {
  it('collapses only established trailing plain-text quote delimiters', () => {
    expect(splitQuotedText('Fresh reply\n\nOn Fri, Aug 14, Alice wrote:\n> Earlier')).toEqual({
      visible: 'Fresh reply',
      quoted: 'On Fri, Aug 14, Alice wrote:\n> Earlier',
      hasQuoted: true,
    })
    expect(splitQuotedText('On-call notes remain visible.\nFrom experience, this works.')).toEqual({
      visible: 'On-call notes remain visible.\nFrom experience, this works.',
      quoted: '',
      hasQuoted: false,
    })
  })

  it('recognizes forwarded messages without matching ordinary subjects', () => {
    expect(isForwardedSubject('Fwd: Quarterly plan')).toBe(true)
    expect(isForwardedSubject('FW: Quarterly plan')).toBe(true)
    expect(isForwardedSubject('Quarterly forward plan')).toBe(false)
  })

  it('finds every conversation occurrence and wraps next/previous selection', () => {
    const messages = [
      { id: 'one', sender: 'Alice', subject: 'Project Atlas', body: 'Atlas is ready.' },
      { id: 'two', sender: 'Bob', subject: 'Re: Project Atlas', body: 'Ship atlas today.' },
    ]
    const first = searchConversation(messages, 'ATLAS', 0)
    expect(first.count).toBe(4)
    expect(first.results.map(result => result.messageId)).toEqual(['one', 'one', 'two', 'two'])
    expect(first.messages[0].findSubjectSegments.some(segment => segment.current)).toBe(true)

    const previous = searchConversation(messages, 'atlas', -1)
    expect(previous.index).toBe(3)
    expect(previous.messages[1].findBodySegments.find(segment => segment.match)?.current).toBe(true)
    expect(searchConversation(messages, '   ', 5)).toMatchObject({ term: '', count: 0, index: -1 })
  })

  it('bounds adversarially large result sets', () => {
    const result = searchConversation([{ id: 'large', body: 'x '.repeat(5000) }], 'x', 5000)
    expect(result.count).toBe(1000)
    expect(result.index).toBe(0)
  })

})
