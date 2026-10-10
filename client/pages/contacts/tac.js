// @ts-check

/** Personal contacts, groups, found-in-mail discovery, and vCard interchange. */
export default class {
  static __tachyonOnMount = ['init']

  get canUseApp() { return !!(window._caduceus?.auth?.isLoggedIn || sessionStorage.getItem('caduceus_token')) }
  get canShowLogin() { return !this.canUseApp }

  /** @type {Array<Record<string, any>>} */
  $contacts = contactCache().contacts
  /** @type {Array<Record<string, any>>} */
  $groups = contactCache().groups
  /** @type {RemoteAddressBook[]} */
  $addressBooks = contactCache().addressBooks
  /** @type {string[]} */
  @SessionState
  checkedIds = []
  @SessionState
  selectedId = ''
  @SessionState
  query = ''
  @SessionState
  sourceFilter = ''
  @SessionState
  groupFilter = ''
  @SessionState
  loading = true
  @SessionState
  editing = false
  @SessionState
  saving = false
  $error = ''
  @SessionState
  conflict = false
  @SessionState
  destination = 'local'
  @SessionState
  groupDialogOpen = false
  @SessionState
  groupEditingId = ''
  $groupName = ''
  @SessionState
  groupError = ''
  /** @type {ContactForm} */
  $form = blankForm()
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _searchTimer
  /** @type {HTMLElement | null} */
  _groupDialogReturnFocus = null
  /** @type {{ id: string, tagName: string, ariaLabel: string, title: string } | null} */
  _groupDialogReturnIdentity = null
  @SessionState
  bootstrapped = false

  get selectedContact() { return this.$contacts.find(contact => contact.id === this.selectedId) || null }
  get pageClass() { return `contacts-page${this.$selectedId || this.$editing ? ' has-selection' : ''}` }
  get contactCountLabel() { return `${this.$contacts.length} ${this.$contacts.length === 1 ? 'contact' : 'contacts'}` }
  get exportLabel() { return this.checkedIds.length ? `Export selected (${this.$checkedIds.length})` : 'Export all' }
  get editorTitle() { return this.selectedId ? 'Edit contact' : 'New contact' }
  get canAssignGroups() { return this.selectedId ? this.selectedContact?.source !== 'carddav' : this.destination === 'local' }
  get canReviewConflict() { return this.conflict && !!this.selectedContact && !this.selectedContact.readOnly }

  init() {}

  async bootstrap() {
    if (this.bootstrapped) return
    this.bootstrapped = true
    await this.loadAll()
  }

  async loadAll() {
    this.loading = true
    await Promise.all([this.loadGroups(), this.loadAddressBooks(), this.loadContacts()])
    this.loading = false
  }

  async loadGroups() {
    const cache = contactCache()
    const response = await window._caduceus?.apiFetch('/contacts/groups')
    cache.groups = response?.ok ? await response.json() : []
    this.$groups = cache.groups
  }

  async loadAddressBooks() {
    const cache = contactCache()
    try {
      const response = await window._caduceus?.apiFetch('/contacts/address-books')
      cache.addressBooks = response?.ok ? await response.json() : []
    } catch {
      cache.addressBooks = []
    }
    this.$addressBooks = cache.addressBooks
    if (this.destination !== 'local' && !this.$addressBooks.some(book => this.destinationValue(book) === this.destination)) this.destination = 'local'
  }

  async loadContacts() {
    const cache = contactCache()
    const sequence = ++cache.loadSequence
    const params = new URLSearchParams()
    if (this.query.trim()) params.set('q', this.query.trim())
    if (this.sourceFilter) params.set('source', this.sourceFilter)
    if (this.groupFilter) params.set('groupId', this.groupFilter)
    try {
      const response = await window._caduceus?.apiFetch(`/contacts${params.size ? `?${params}` : ''}`)
      const contacts = response?.ok ? await response.json() : []
      if (cache !== contactCache() || sequence !== cache.loadSequence) return
      cache.contacts = contacts
      this.$contacts = cache.contacts
      this.checkedIds = this.checkedIds.filter(id => this.$contacts.some(contact => contact.id === id))
      if (this.selectedId && !this.$contacts.some(contact => contact.id === this.selectedId)) this.selectedId = ''
    } catch {
      if (cache === contactCache() && sequence === cache.loadSequence) {
        cache.contacts = []
        this.$contacts = cache.contacts
        this.$error = 'Contacts could not be loaded.'
      }
    }
  }

  onSearchInput(event) {
    this.query = eventValue(event)
    clearTimeout(this._searchTimer)
    this._searchTimer = setTimeout(() => void this.loadContacts(), 180)
  }

  async setSourceFilter(source) {
    this.sourceFilter = String(source || '')
    this.groupFilter = ''
    this.selectedId = ''
    await this.loadContacts()
  }

  async setGroupFilter(event) {
    const groupId = eventData(event, '[data-contact-group-id]')?.contactGroupId
    if (!groupId) return
    this.groupFilter = String(groupId || '')
    this.sourceFilter = ''
    this.selectedId = ''
    await this.loadContacts()
  }

  filterClass(kind, value = '') {
    const active = kind === 'source'
      ? this.sourceFilter === value && !this.groupFilter
      : this.groupFilter === value
    return `contacts-filter${active ? ' active' : ''}`
  }

  contactRowClass(contact) { return `contact-row${contact.id === this.$selectedId ? ' active' : ''}` }
  isChecked(id) { return this.checkedIds.includes(id) }

  toggleChecked(event) {
    const target = /** @type {HTMLInputElement | null} */ (event?.target)
    const id = target?.dataset?.contactId
    if (!id) return
    const checked = Boolean(target.checked)
    this.checkedIds = checked
      ? [...new Set([...this.checkedIds, id])]
      : this.checkedIds.filter(current => current !== id)
  }

  selectContact(event) {
    const id = eventData(event, '[data-contact-id]')?.contactId
    if (!id) return
    this.selectedId = id
    this.editing = false
	this.$error = ''
	this.conflict = false
  }

  closeDetail() {
    this.selectedId = ''
    this.editing = false
	this.$error = ''
	this.conflict = false
  }

  openNew() {
    this.selectedId = ''
    this.$form = blankForm()
	this.editing = true
	this.$error = ''
	this.conflict = false
	this.destination = 'local'
  }

  startEdit() {
    if (!this.selectedContact || this.selectedContact.readOnly) return
    this.$form = editableContact(this.selectedContact)
	this.editing = true
	this.$error = ''
	this.conflict = false
  }

  cancelEdit() {
    this.editing = false
    this.$error = ''
    if (!this.selectedId) this.closeDetail()
  }

  updateDestination(event) {
    const value = eventValue(event)
    this.destination = value === 'local' || this.$addressBooks.some(book => this.destinationValue(book) === value) ? value : 'local'
    if (this.destination !== 'local') this.$form = { ...this.$form, groupIds: [] }
  }

  destinationValue(book) { return `${book.accountId}|${book.addressBookId}` }
  destinationLabel(book) { return `${book.accountLabel || book.mailboxAddress || 'External account'} — ${book.name || 'Contacts'}` }

  dismissOperationError() {
    this.$error = ''
    this.conflict = false
  }

  reviewConflict() {
    if (!this.canReviewConflict) return
    this.startEdit()
  }

  updateFormField(event) {
    const field = /** @type {HTMLElement | null} */ (event?.target)?.dataset?.field
    if (!field) return
    this.$form = { ...this.$form, [field]: eventValue(event) }
  }

  updateValue(event) {
    const dataset = /** @type {HTMLElement | null} */ (event?.target)?.dataset
    const kind = dataset?.kind
    const key = dataset?.key
    const field = dataset?.field
    if (!kind || !key || !field || !Array.isArray(this.$form[kind])) return
    this.$form = {
      ...this.$form,
      [kind]: this.$form[kind].map(item => item.key === key ? { ...item, [field]: eventValue(event) } : item),
    }
  }

  addValue(kind) {
    this.$form = { ...this.$form, [kind]: [...this.$form[kind], editableValue()] }
  }

  removeValue(event) {
    const dataset = eventData(event, '[data-value-key]')
    const kind = dataset?.kind
    const key = dataset?.valueKey
    if (!kind || !key || !Array.isArray(this.$form[kind])) return
    const values = this.$form[kind].filter(item => item.key !== key)
    this.$form = { ...this.$form, [kind]: values.length ? values : [editableValue()] }
  }

  updateAddress(event) {
    const dataset = /** @type {HTMLElement | null} */ (event?.target)?.dataset
    const key = dataset?.key
    const field = dataset?.field
    if (!key || !field) return
    this.$form = {
      ...this.$form,
      addresses: this.$form.addresses.map(address => address.key === key
        ? { ...address, [field]: eventValue(event) }
        : address),
    }
  }

  addAddress() { this.$form = { ...this.$form, addresses: [...this.$form.addresses, editableAddress()] } }

  removeAddress(event) {
    const key = eventData(event, '[data-address-key]')?.addressKey
    if (!key) return
    const addresses = this.$form.addresses.filter(address => address.key !== key)
    this.$form = { ...this.$form, addresses: addresses.length ? addresses : [editableAddress()] }
  }

  toggleFormGroup(event) {
    const target = /** @type {HTMLInputElement | null} */ (event?.target)
    const groupId = target?.dataset?.groupId
    if (!groupId) return
    const checked = Boolean(target.checked)
    const current = this.$form.groupIds || []
    this.$form = {
      ...this.$form,
      groupIds: checked ? [...new Set([...current, groupId])] : current.filter(id => id !== groupId),
    }
  }

  formHasGroup(groupId) { return this.$form.groupIds.includes(groupId) }

  async saveContact() {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch) return
	const body = /** @type {any} */ (contactPayload(this.$form))
	const remoteBook = !this.selectedId && this.destination !== 'local'
	  ? this.$addressBooks.find(book => this.destinationValue(book) === this.destination)
	  : null
	if (!this.selectedId && this.destination !== 'local' && !remoteBook) {
	  this.$error = 'Choose an available address book.'
	  return
	}
	if (remoteBook) Object.assign(body, { source: 'carddav', accountId: remoteBook.accountId, addressBookId: remoteBook.addressBookId, idempotencyKey: mutationKey('create'), groupIds: [] })
	if (this.selectedContact?.source === 'carddav') body.idempotencyKey = mutationKey('update')
    if (!body.name && body.emails.length === 0 && body.phones.length === 0) {
      this.$error = 'Add a name, email address, or phone number.'
      return
    }
	this.saving = true
	this.$error = ''
	this.conflict = false
    try {
      const response = await apiFetch(this.selectedId ? `/contacts/${this.$selectedId}` : '/contacts', {
        method: this.selectedId ? 'PUT' : 'POST',
        body: JSON.stringify(body),
      })
      const data = await response?.json()
      if (!response?.ok) { this.$error = data?.error || 'Contact could not be saved.'; return }
	  const mutationContactId = data.id
	  this.selectedId = mutationContactId
	  this.editing = false
	  if (data.syncState && data.mutationId && data.accountId) {
		const outcome = await this.waitForCardDAVMutation({ ...data, contactId: mutationContactId })
		if (outcome === 'conflict' || outcome === 'failed') {
		  await this.loadContacts()
		  this.showMutationFailure(outcome, remoteBook
			? 'The CardDAV server already has a different contact at the new resource location.'
			: 'This contact changed on the CardDAV server. The server version was kept.')
		  return
        }
      }
      await this.loadContacts()
      window._caduceus?.toast('Contact saved.')
    } catch {
      this.$error = 'Network error while saving the contact.'
    } finally {
      this.saving = false
    }
  }

  async deleteSelected() {
    const contact = this.selectedContact
    if (!contact || !confirm(`Delete ${contact.name || contact.emails?.[0]?.value || 'this contact'}?`)) return
    const response = await window._caduceus?.apiFetch(`/contacts/${contact.id}?idempotencyKey=${encodeURIComponent(mutationKey('delete'))}`, { method: 'DELETE' })
    if (!response?.ok) { window._caduceus?.toast('Contact could not be deleted.'); return }
    const result = await response.json().catch(() => ({}))
    if (result?.pending) {
	  const outcome = await this.waitForCardDAVMutation({ mutationId: result.mutationId, accountId: result.accountId, contactId: contact.id })
	  if (outcome === 'conflict' || outcome === 'failed') {
		await this.loadContacts()
		this.showMutationFailure(outcome, 'This contact changed on the CardDAV server and was not deleted. The server version was kept.')
		return
      }
    }
    this.checkedIds = this.checkedIds.filter(id => id !== contact.id)
    this.closeDetail()
    await this.loadContacts()
    window._caduceus?.toast('Contact deleted.')
  }

  composeToContact() {
    const contact = this.selectedContact
    const email = contact?.emails?.[0]?.value
    if (email) window._caduceus?.compose({ to: email })
  }

  showMutationFailure(outcome, conflictMessage) {
	this.conflict = outcome === 'conflict'
	this.$error = this.conflict ? conflictMessage : 'The CardDAV server rejected this contact change.'
  }

  async waitForCardDAVMutation({ mutationId, accountId, contactId }) {
    const apiFetch = window._caduceus?.apiFetch
    if (!apiFetch || !mutationId || !accountId) return 'queued'
    for (let attempt = 0; attempt < 16; attempt++) {
      if (attempt) await new Promise(resolve => setTimeout(resolve, 500))
	  const response = await apiFetch(`/contacts/${encodeURIComponent(contactId || this.$selectedId)}/mutations/${encodeURIComponent(mutationId)}?accountId=${encodeURIComponent(accountId)}`)
      if (!response?.ok) continue
      const mutation = await response.json()
      if (mutation.state === 'applied' || mutation.state === 'conflict' || mutation.state === 'failed') return mutation.state
    }
    window._caduceus?.toast('Contact change is queued for synchronization.')
    return 'queued'
  }

  /** @param {Event | null} [event] */
  openGroupDialog(event = null) {
    const active = event?.currentTarget instanceof HTMLElement
      ? event.currentTarget
      : document.activeElement instanceof HTMLElement ? document.activeElement : null
    this._groupDialogReturnFocus = active
    this._groupDialogReturnIdentity = active ? {
      id: active.id,
      tagName: active.tagName.toLowerCase(),
      ariaLabel: active.getAttribute('aria-label') || '',
      title: active.getAttribute('title') || '',
    } : null
    const groupId = eventData(event, '[data-contact-group-id]')?.contactGroupId
    const group = groupId ? this.$groups.find(item => item.id === groupId) : null
    this.groupEditingId = group?.id || ''
    this.$groupName = group?.name || ''
    this.groupError = ''
    this.groupDialogOpen = true
    requestAnimationFrame(() => requestAnimationFrame(() => {
      const input = document.querySelector('[data-contact-group-name]')
      if (input instanceof HTMLElement) input.focus()
    }))
  }

  closeGroupDialog() {
    const restore = this._groupDialogReturnFocus
    const identity = this._groupDialogReturnIdentity
    this._groupDialogReturnFocus = null
    this._groupDialogReturnIdentity = null
    this.groupDialogOpen = false
    this.groupEditingId = ''
    this.$groupName = ''
    this.groupError = ''
    void (/** @type {any} */ (globalThis)).__tc_rerender?.().then(() => {
      if (restore?.isConnected) { restore.focus(); return }
      if (!identity) return
      if (identity.id) {
        const byId = document.getElementById(identity.id)
        if (byId instanceof HTMLElement) { byId.focus(); return }
      }
      const replacement = [...document.querySelectorAll(identity.tagName)].find(candidate =>
        (!identity.ariaLabel || candidate.getAttribute('aria-label') === identity.ariaLabel) &&
        (!identity.title || candidate.getAttribute('title') === identity.title)
      )
      if (replacement instanceof HTMLElement) replacement.focus()
    })
  }

  groupDialogKeydown(event) {
    if (event?.key === 'Escape') {
      event.preventDefault()
      this.closeGroupDialog()
      return
    }
    if (event?.key !== 'Tab') return
    const dialog = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
    if (!dialog) return
    const focusable = [...dialog.querySelectorAll('button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])')]
      .filter(element => element instanceof HTMLElement && !element.hidden && element.getClientRects().length > 0)
    const first = focusable[0]
    const last = focusable.at(-1)
    if (!(first instanceof HTMLElement) || !(last instanceof HTMLElement)) return
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  stopPropagation(event) { event?.stopPropagation?.() }

  updateGroupName(event) { this.$groupName = eventValue(event) }

  async saveGroup() {
    const name = this.$groupName.trim()
    if (!name) { this.groupError = 'Group name is required.'; return }
    const response = await window._caduceus?.apiFetch(
      this.groupEditingId ? `/contacts/groups/${this.$groupEditingId}` : '/contacts/groups',
      { method: this.groupEditingId ? 'PUT' : 'POST', body: JSON.stringify({ name }) },
    )
    const data = await response?.json()
    if (!response?.ok) { this.groupError = data?.error || 'Group could not be saved.'; return }
    this.closeGroupDialog()
    await this.loadGroups()
    window._caduceus?.toast('Group saved.')
  }

  async deleteGroup(event) {
    const groupId = eventData(event, '[data-contact-group-id]')?.contactGroupId
    const group = this.$groups.find(item => item.id === groupId)
    if (!group) return
    if (!confirm(`Delete the “${group.name}” group? Contacts will be kept.`)) return
    const response = await window._caduceus?.apiFetch(`/contacts/groups/${group.id}`, { method: 'DELETE' })
    if (!response?.ok) { window._caduceus?.toast('Group could not be deleted.'); return }
    if (this.groupFilter === group.id) this.groupFilter = ''
    await Promise.all([this.loadGroups(), this.loadContacts()])
    window._caduceus?.toast('Group deleted. Contacts were kept.')
  }

  pickImport() { /** @type {HTMLInputElement | null} */ (document.querySelector('[data-contact-import]'))?.click() }

  async importVCard(event) {
    const input = /** @type {HTMLInputElement} */ (event.target)
    const file = input.files?.[0]
    if (!file) return
    try {
      const response = await window._caduceus?.apiFetch('/contacts/import', {
        method: 'POST',
        body: JSON.stringify({ content: await file.text(), groupIds: this.groupFilter ? [this.groupFilter] : [] }),
      })
      const data = await response?.json()
      if (!response?.ok) { window._caduceus?.toast(data?.error || 'vCard import failed.'); return }
      await this.loadContacts()
      window._caduceus?.toast(`Imported ${data.imported} ${data.imported === 1 ? 'contact' : 'contacts'}${data.skipped ? `; skipped ${data.skipped}` : ''}.`)
    } catch {
      window._caduceus?.toast('vCard import failed.')
    } finally {
      input.value = ''
    }
  }

  async exportContacts() {
    const ids = [...this.checkedIds]
    const response = await window._caduceus?.apiFetch('/contacts/export', {
      method: 'POST', body: JSON.stringify({ ids }),
    })
    const data = await response?.json()
    if (!response?.ok || !data?.contentBase64) { window._caduceus?.toast(data?.error || 'vCard export failed.'); return }
    const bytes = Uint8Array.from(atob(data.contentBase64), character => character.charCodeAt(0))
    const url = URL.createObjectURL(new Blob([bytes], { type: data.contentType || 'text/vcard;charset=utf-8' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = data.filename || 'contacts.vcf'
    anchor.click()
    setTimeout(() => URL.revokeObjectURL(url), 0)
    window._caduceus?.toast(`Exported ${data.count} ${data.count === 1 ? 'contact' : 'contacts'}.`)
  }

  initials(contact) {
    const source = String(contact?.name || contact?.emails?.[0]?.value || '?').trim()
    const parts = source.split(/[@\s._-]+/).filter(Boolean)
    return (parts.length > 1 ? `${parts[0][0]}${parts[1][0]}` : source.slice(0, 2)).toUpperCase()
  }

  primaryEmail(contact) { return contact?.emails?.[0]?.value || '' }
  primaryPhone(contact) { return contact?.phones?.[0]?.value || '' }
  sourceLabel(source) { return source === 'mail' ? 'Found in mail' : source === 'vcard' ? 'Imported vCard' : source === 'carddav' ? 'Synced address book' : 'Personal' }
  groupName(id) { return this.$groups.find(group => group.id === id)?.name || '' }
  addressLabel(address) { return [address.streetAddress, address.extendedAddress, address.city, address.region, address.postalCode, address.country].filter(Boolean).join(', ') }
}

function eventValue(event) {
  const target = /** @type {HTMLInputElement | HTMLTextAreaElement | null} */ (event?.target)
  return String(event?.value ?? target?.value ?? '')
}

function eventData(event, selector) {
  const target = event?.target instanceof Element ? event.target.closest(selector) : null
  return /** @type {HTMLElement | null} */ (target)?.dataset
}

function contactCache() {
  const owner = String(sessionStorage.getItem('caduceus_email') || '').toLowerCase()
  if (window._caduceusContactsCache?.owner !== owner) {
	window._caduceusContactsCache = { owner, contacts: [], groups: [], addressBooks: [], loadSequence: 0 }
  }
	window._caduceusContactsCache.addressBooks ||= []
  return window._caduceusContactsCache
}

function editableValue(value = {}) {
  return { key: crypto.randomUUID(), value: String(value.value || ''), type: String(value.type || '') }
}

function editableAddress(value = {}) {
  return {
    key: crypto.randomUUID(), type: String(value.type || ''), streetAddress: String(value.streetAddress || ''),
    extendedAddress: String(value.extendedAddress || ''), city: String(value.city || ''), region: String(value.region || ''),
    postalCode: String(value.postalCode || ''), country: String(value.country || ''),
  }
}

/** @returns {ContactForm} */
function blankForm() {
  return {
    name: '', givenName: '', familyName: '', company: '', title: '', birthday: '', notes: '', nicknames: '',
    emails: [editableValue()], phones: [editableValue()], urls: [editableValue()], addresses: [editableAddress()], groupIds: /** @type {string[]} */ ([]),
  }
}

/** @returns {ContactForm} */
function editableContact(contact) {
  const form = blankForm()
  return {
    ...form,
    name: contact.name || '', givenName: contact.givenName || '', familyName: contact.familyName || '',
    company: contact.company || '', title: contact.title || '', birthday: contact.birthday || '', notes: contact.notes || '',
    nicknames: (contact.nicknames || []).join(', '),
    emails: contact.emails?.length ? contact.emails.map(editableValue) : form.emails,
    phones: contact.phones?.length ? contact.phones.map(editableValue) : form.phones,
    urls: contact.urls?.length ? contact.urls.map(editableValue) : form.urls,
    addresses: contact.addresses?.length ? contact.addresses.map(editableAddress) : form.addresses,
    groupIds: [...(contact.groupIds || [])],
  }
}

function contactPayload(form) {
  const values = items => items
    .map(({ value, type }) => ({ value: String(value || '').trim(), ...(String(type || '').trim() ? { type: String(type).trim() } : {}) }))
    .filter(item => item.value)
  const addresses = form.addresses.map(({ key: _key, ...address }) => Object.fromEntries(
    Object.entries(address).map(([name, value]) => [name, String(value || '').trim()]).filter(([, value]) => value),
  )).filter(address => Object.keys(address).some(key => key !== 'type'))
  return {
    name: form.name.trim(), givenName: form.givenName.trim(), familyName: form.familyName.trim(),
    company: form.company.trim(), title: form.title.trim(), birthday: form.birthday.trim(), notes: form.notes.trim(),
    nicknames: form.nicknames.split(',').map(value => value.trim()).filter(Boolean),
    emails: values(form.emails), phones: values(form.phones), urls: values(form.urls), addresses,
    groupIds: [...form.groupIds],
  }
}

function mutationKey(kind) { return `contact-${kind}-${crypto.randomUUID()}` }
