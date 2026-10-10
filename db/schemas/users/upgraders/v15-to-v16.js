/** Add the complete MailSpring-compatible web preference surface. */
export default function upgrade(doc) {
  return {
    ...doc,
    mailPreferences: {
      workspaceLayout: 'split', use24HourClock: false, interfaceScale: 1,
      showUnreadForAllFolders: false, markAsReadDelay: 500,
      detailedHeaders: false, detailedNames: false, restrictMessageWidth: false,
      backspaceAction: 'archive', swipeEnabled: true, conversationOrder: 'ascending',
      emailRenderMode: 'theme', richText: true, spellcheck: true, spellcheckLanguage: '',
      defaultSendAction: 'send', defaultReplyAction: 'reply', undoSendDelay: 5000,
      defaultFrom: '', autoCcBySender: {}, autoBccBySender: {},
      displayAttachmentPreviews: true, unsnoozeToTop: true, findInMailDisabledDomains: [],
    },
  }
}
