import { describe, expect, it } from 'bun:test'
import { ComposerTemplatesUtils } from '../../client/shared/utils/ComposerTemplatesUtils.js'
import { TemplateContent } from '../../server/shared/mailbox/TemplateContent.js'

const hasEmptyTemplateVariables = (...args) => ComposerTemplatesUtils.hasEmptyTemplateVariables(...args)
const insertTemplateBeforeProtectedContent = (...args) => ComposerTemplatesUtils.insertTemplateBeforeProtectedContent(...args)
const normalizeTemplateVariables = (...args) => ComposerTemplatesUtils.normalizeTemplateVariables(...args)
const sanitizeTemplateHTML = (...args) => ComposerTemplatesUtils.sanitizeTemplateHTML(...args)
const stripTemplateVariableMarkup = (...args) => ComposerTemplatesUtils.stripTemplateVariableMarkup(...args)
const templateBodyFromDraft = (...args) => ComposerTemplatesUtils.templateBodyFromDraft(...args)

describe('Mailspring-compatible composer templates', () => {
  it('normalizes editable code regions and detects truly empty blanks', () => {
    const html = normalizeTemplateVariables('<p>Hello <code class="var">Name</code> <code class="var"><br></code></p>')
    expect(html).toContain('class="var" data-template-var="true">Name</code>')
    expect(html).toContain('class="var empty" data-template-var="true"><br></code>')
    expect(hasEmptyTemplateVariables(html)).toBe(true)
    expect(hasEmptyTemplateVariables('<code class="var">Filled</code>')).toBe(false)
  })

  it('strips only authoring wrappers before delivery', () => {
    expect(stripTemplateVariableMarkup('<p>Hello <code class="var empty" data-template-var="true">Ada</code>.</p>'))
      .toBe('<p>Hello Ada.</p>')
    expect(stripTemplateVariableMarkup('<p>Use <code>npm test</code>.</p>'))
      .toBe('<p>Use <code>npm test</code>.</p>')
  })

  it('replaces editable reply content while preserving signature and quote', () => {
    const current = '<p>Old body</p><div class="caduceus-signature">Regards</div><blockquote class="gmail_quote">Earlier</blockquote>'
    expect(insertTemplateBeforeProtectedContent('<p>New body</p>', current))
      .toBe('<p>New body</p><div class="caduceus-signature">Regards</div><blockquote class="gmail_quote">Earlier</blockquote>')
    expect(templateBodyFromDraft(current)).toBe('<p>Old body</p>')
  })

  it('removes active content and executable attributes at both trust boundaries', () => {
    const hostile = '<script>alert(1)</script><p onclick="steal()"><a href="javascript:steal()">Safe text</a></p>'
    for (const sanitized of [sanitizeTemplateHTML(hostile), TemplateContent.sanitizeStoredTemplateHTML(hostile)]) {
      expect(sanitized).not.toMatch(/script|onclick|javascript:/i)
      expect(sanitized).toContain('Safe text')
    }
  })

  it('validates bounded input and generates Mailspring-style duplicate suffixes', () => {
    expect(TemplateContent.normalizeTemplateInput(null)).toBeInstanceOf(Error)
    expect(TemplateContent.normalizeTemplateInput({ name: 'Bad/name', text: 'Body' })).toBeInstanceOf(Error)
    expect(TemplateContent.normalizeTemplateInput({ name: ' Follow-up ', text: '<p>Body</p>' })).toEqual({
      name: 'Follow-up', subject: '', text: '<p>Body</p>', to: '', cc: '',
    })
    expect(TemplateContent.resolveUniqueTemplateName('Follow-up', [{ id: '1', name: 'follow-up' }, { id: '2', name: 'Follow-up 1' }]))
      .toBe('Follow-up 2')
  })
})
