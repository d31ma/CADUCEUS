# Mesail product specification

- Status: Draft for implementation
- Date: 2026-11-10
- Owner: Senior Product Manager (Mesail rewrite)
- Architecture: [ADR 0004](../adr/0004-mesail-go-serverless-keycloak.md)
- Slices: [SLICES.md](SLICES.md)
- Domain model and contracts: [DOMAIN.md](DOMAIN.md), [CONTRACTS.md](CONTRACTS.md)
- UX: the screens, states, copy and HTML mockups are in
  [`docs/mesail/ux/`](ux/) (owned by the mesail-ux session). This SPEC is
  the source for behavior. When the two disagree, this SPEC wins and the UX
  files are fixed.
- Hosting: on-premise first (ADR 0007): every feature runs on the owner's
  own machines, with no cloud account or cloud service. The services are
  built so they can also run as Lambda container images (Floci locally,
  a cloud later); on-premise they run as Lambda images in the Floci
  emulator on kestrel (Q18; ADR 0004, ADR 0007).

## 1. What Mesail is

Mesail is one application that is both a team messenger and an email client
and server. It replaces CADUCEUS.

- The interface works like a team messenger (Slack or Microsoft Teams):
  channels, direct messages and threads.
- Mesail hosts mail for several domains, as CADUCEUS did. **Each hosted mail
  domain is also one messaging space.** The domain is the tenant. All chat
  (channels, direct messages, threads, reactions, read state, presence and
  search) stays inside one domain. There is no organization and no
  workspace above or inside a domain.
- The same interface sends and receives email. A message to a member of the
  sender's own domain is delivered as a chat message by default. A message
  to any other address is delivered as an email. This includes an address
  of another domain that the same Mesail install hosts. The user sees this
  choice for every recipient before sending, and can change it where it is
  allowed.
- Email works between any domains: between two domains of the same install,
  and to and from outside addresses. Chat never crosses a domain.
- Email that arrives shows up as a conversation, next to chat conversations,
  with the same reading and reply experience.
- **Offline first** (ADR 0006): on a device the person keeps signed in,
  Mesail keeps a copy of their recent conversations and mail, works
  offline (read, search, write), queues what they write and sends it when
  the device is online again. The server stays the source of truth.
- Sign-in and identity come from Keycloak (OpenID Connect), reached only by
  Mesail's backend. A person has **one login** for every domain: their
  username is their email prefix (the part before `@`), which is the same
  in all their domains. After sign-in they switch between their domains
  without signing in again. Mesail stores no passwords, passkeys or second
  factors of its own (ADR 0005).

## 2. Terms

These terms are used the same way in the spec, the code and the API.

| Term | Meaning |
| --- | --- |
| Domain | A mail domain that the install hosts, such as `example.com`. It is the tenant and the messaging space. It has DNS state (MX, SPF, DKIM, DMARC), DKIM keys, its members, aliases and channels. No chat data crosses domains. |
| Install | One running Mesail deployment. It hosts one or more domains. An Operator runs it. |
| Person | One human with one Mesail login: one Keycloak user in the realm `mesail`, with one password and one 2-step code. Identified by the person id (the Keycloak `sub`). A person belongs to the domains an admin added them to, and to no other. |
| Username (prefix) | A person's username: the local part (before `@`) of all their member addresses, for example `ana`. It is reserved on the whole install: no other person, alias or channel address may use it in any domain. |
| Member | A person in one domain. Identified by the domain and the person id (`memberId` = person id, the same in every domain). In a domain the member's primary address is `<username>@<domain>`, and the member may have aliases in that domain. |
| Active domain | The one domain a signed-in session works in. The access token names it; every request is scoped to it. |
| Address | An email address. It is *internal* (same domain) when it belongs to a member, alias or channel of the sender's domain. Every other address is not internal: either an address of another domain hosted by this install, or an external address. |
| Recipient kind | The result of resolving one recipient address for one sender. Exact API values: `member` (a member or alias of the sender's domain), `channel` (a channel address of the sender's domain), `otherDomain` (a member, alias or channel address of another domain hosted by this install, counting only `active` domains), `external` (any other address). |
| Channel | A named conversation in a domain. Channels belong to the domain directly. Public (any member of the domain may join) or private (invite only). A channel may have an email address in its domain, so mail can be sent into it. |
| Direct conversation | A conversation between a fixed set of participants, outside any channel. With one other participant it is a direct message (DM); with more it is a group DM. Chat participants are members of the same domain. Other participants take part by email. |
| Conversation | A channel or a direct conversation. Every conversation belongs to exactly one domain. Every message belongs to exactly one conversation. |
| Message | One post in a conversation. It has an author, a body, optional attachments, and, when email was involved, email headers. |
| Thread | The replies to one message (the thread root). Replies stay in the same conversation. |
| Delivery | How one recipient receives one message: `chat` or `email`. Each delivery has its own status. |
| Email conversation | A direct conversation that has at least one participant who takes part by email (an `otherDomain` or `external` address, or a member switched to email). It is still a conversation; email is how those participants take part. |
| Device copy | The recent conversations, messages and lists of one person and one domain that a kept device stores, so Mesail works offline (ADR 0006). The server's data always wins over the copy. |
| Kept device | A device where the person ticked "Keep me signed in on this device" at sign-in. Only a kept device has a device copy and a session that survives closing the app. |
| Queue | The changes a person made on a device that are not yet accepted by the server: sends, edits, deletions, reactions, read positions. Each shows "Waiting to send" until accepted, or "Needs your attention" when refused. |
| Domain switcher | The control at the top of the interface that lists the person's domains and changes the active domain of the current tab, without a password. |

## 3. Actors

- **Member**: reads and writes messages in their own domain, manages their
  own settings. Can create channels (AC-CH-1).
- **DomainAdmin** (a member marked as admin of the domain in `directory`;
  token role `mesail-admin` while that domain is active): manages one
  domain: adds and removes its members, aliases, channels, channel email
  addresses, retention and mail policy for that domain. A DomainAdmin has
  no rights in any other domain, and may reset a person's credentials only
  when they administer every domain of that person (AC-ID-13).
- **Operator** (Keycloak realm role `mesail-operator` in the realm
  `mesail`): runs the install. Adds and removes domains, names the first
  DomainAdmin of a domain, and manages people (AC-ID-13, AC-ID-14). An
  Operator does not read the messages of a domain through this role.
- **Service**: a backend service that calls another service. It uses a
  service token, never a user token, and has no user rights.
- **Correspondent in another domain**: a member of another domain hosted by
  the same install. For the sender, this person is an email correspondent
  only.
- **External correspondent**: anyone with an address that the install does
  not host. They use their own email client. They never see the Mesail
  interface.

## 4. User scenarios

### S1. Chat in a channel
Ana is a member of `example.com`. She selects `#releases` and writes
"Build 42 is green". Every member of `#releases` who is online sees the
message within 2 seconds, without reloading. Bo replies in a thread. The
channel shows "1 reply" under Ana's message, and Ana gets a notification.

### S2. Direct message to a colleague
Ana starts a new conversation and types `bo@example.com`. Mesail
recognizes Bo as a member of her domain (`member`) and shows the recipient
chip as **Chat**. Ana writes and sends. Bo sees a DM, not an email.

### S3. Email to an outside person from the same composer
Ana starts a new conversation and types `carol@partner.org`. The chip shows
**Email**. Because there is an email recipient, the composer shows a
*Subject* field and the *Cc* and *Bcc* options. Ana writes a subject and a
body and sends. Carol receives a normal email from `ana@example.com`. In
Mesail the conversation looks like a DM with Carol, marked with an email
icon.

### S4. Mixed recipients
Ana writes to `bo@example.com` and `carol@partner.org` together. Bo's chip
shows **Chat** and Carol's shows **Email**. The composer explains in one
line: "Bo gets a chat message. Carol gets an email." Ana sends once. Bo
sees the message in a group conversation that also shows Carol as an email
participant. Carol gets one email with Ana on the From line and, unless Ana
chose otherwise, Bo on the Cc line, so that Carol's reply-all reaches Bo.

### S5. Changing the choice for a recipient
Ana wants Bo to get a real email too (for his records). She opens Bo's chip
and picks **Send as email**. The chip changes to **Email**. Bo now gets the
message both in Mesail and as an email in his mailbox. Ana cannot pick
**Chat** for Carol: the option is shown but disabled, with the reason
"carol@partner.org has no Mesail account".

### S6. Reply from outside
Carol replies to Ana's email from her own email client. The reply appears in
the same Mesail conversation, under Ana's message, within the time the mail
system takes to deliver it. Bo sees it too, because he was a participant.

### S7. New email from outside
Dan, who never wrote before, emails `ana@example.com`. A new email
conversation with Dan appears in Ana's sidebar under *Email*, marked unread.
Ana answers in the composer. The reply goes to Dan as email, threaded
correctly in his client.

### S8. Email into a channel
`support@example.com` is the email address of `#support` in `example.com`.
A customer writes to it. The email appears in `#support` as a new message
from the customer, with its subject as a bold first line. A member replies
in its thread. The reply goes to the customer as email from
`support@example.com`; other thread replies that are not marked "send to
customer" stay internal.

### S9. Search
Ana types "invoice march" in search. She gets messages and emails she is
allowed to see, from every conversation of her domain, newest first, with
the matching words highlighted. She never gets a result from another
domain, even one hosted by the same install. She can narrow the result to
one conversation, one person, a date range, or "has attachment".

### S10. Presence and notifications
Bo's avatar shows a green dot while he has Mesail open, yellow when idle
for 10 minutes, and a red sign when he set *Do not disturb*. Only members
of `example.com` see Bo's presence. When Ana mentions `@bo` while Bo is
away, Bo gets a browser push notification (if he turned it on in this
browser; AC-PN-3). If he does not read it within the time
he chose (default 30 minutes), he gets one email summary.

### S11. An Operator adds a domain
An Operator adds `example.org` to the install. Mesail adds it as a hosted
domain with an empty messaging space; Keycloak does not change. Mesail shows
the DNS records to create (MX, SPF, DKIM, DMARC), checks them, and shows
each as *ok* or *missing*. Mail for the domain is accepted only after MX and
DKIM are *ok*. The Operator then names the first DomainAdmin of
`example.org` by username: an existing person or a new one (AC-ID-12).

### S12. Sign in and switch domains
A visitor opens Mesail and sees Mesail's own sign-in form with one field,
"Username or email address", a password field and, when needed, a
one-time code. Ana types `ana` (or `ana@example.com`, in any letter case),
her password and her code. Mesail checks them with Keycloak on the server;
the browser never talks to Keycloak. Passkeys are not offered in the first
release. Ana is a member of `example.com` and `example.org`. She lands in
her default domain, `example.com` (had she typed `ana@example.org`, she
would land in `example.org`). The domain switcher at the top lists both
domains. She picks `example.org` and sees its channels, DMs and email,
without signing in again. A second tab can stay in `example.com`. Signing
out in Mesail ends the Keycloak session. An admin who removes Ana from
`example.org` cuts that domain off within 60 seconds; an Operator who
disables Ana's account cuts every domain off within 60 seconds, including
open realtime connections.

### S13. Writing to another domain on the same install
Ana (`ana@example.com`) writes to `eve@example.org`. The same install hosts
`example.org`, and Eve is a member there. The resolver returns
`otherDomain`. The chip shows **Email · other domain**. The Chat option is
shown but disabled, with the reason "Chat stays inside example.com". The
composer shows Subject, Cc and Bcc. Ana sends. Eve gets an email in her
`example.org` account, in a new email conversation, not a chat message. Eve
replies from Mesail. Her reply goes back as email and joins Ana's email
conversation in `example.com`. Neither side sees the other's presence,
channels or search results.

### S14. An admin adds and removes members
Bo is a DomainAdmin of `example.com`. In the admin app he adds the username
`dan` with the display name "Dan Example" and a first password. No person
has `dan` yet, so Mesail creates Dan's account. Dan signs in with `dan`,
must choose a new password first, and has the address `dan@example.com`.
Bo then adds `eve`. Eve already has an account (she is a member of
`example.org`); the admin app says so, and does not ask for a password.
Within a minute Eve sees `example.com` in her domain switcher, with the
address `eve@example.com`. No one can create the alias `dan@example.org`
or a channel address `dan@...` in any domain: `dan` is Dan's prefix
everywhere. Bo removes Eve from `example.com`. Within a minute she no
longer sees `example.com`, and mail to `eve@example.com` is refused. Her
account and `eve@example.org` are unchanged.

### S15. Reading and writing offline
Ana signed in on her phone with "Keep me signed in on this device". On a
train with no network she opens Mesail. She sees her conversations and
mail of the last 14 days and the threads in them (and, once attachments
are kept on devices, the pictures she opened before; ADR 0006 T2). The top of the screen says "Offline - updated 08:12". She searches
for "budget" and gets the hits on this device, marked "On this device".
She replies in #design, edits a message she sent yesterday, adds a
reaction and writes a new email to `carol@partner.test`. Each shows
"Waiting to send". She switches to `example.org` and reads it too. Back
online, everything is sent in the order she wrote it, the new messages of
the last hours appear, and her reply in #design shows the time it reached
the server with "Written offline at 09:40".

### S16. A queued item cannot be sent
While Ana was offline, `bo@example.com` was removed from `example.com`.
Her queued chat message to Bo comes back with "Bo is no longer in
example.com". It stays on her phone under "Needs your attention", with
Edit, Send by email and Discard. Nothing was sent. Her other queued items
were sent. Later an admin removes Ana from `example.org`. The next time
her phone is online it deletes the copy of `example.org` from the phone,
and tells her once that 2 queued items for `example.org` were not sent.

### S17. Signing out on a shared computer
Bo signs in on a library computer without "Keep me signed in on this
device". Mesail keeps nothing on that computer after the tab closes. At
home he signs in with the box ticked; Mesail keeps a copy there. When he
signs out at home with 1 message still queued, Mesail warns him, and
after he confirms it deletes every Mesail copy of his on that computer.

## 5. Acceptance criteria

Each criterion is testable from the outside (HTTP, SMTP, browser). IDs are
stable; tests name them.

### Identity and access (AC-ID)
- **AC-ID-1** Every API route requires a valid Mesail access token, except
  the routes listed as public in [CONTRACTS.md](CONTRACTS.md) (health,
  sign-in, refresh, sign-out, domain switch, the token key set and the
  service token route). A missing, expired, wrongly signed or
  wrong-audience token gets `401` with no data.
- **AC-ID-2** One Keycloak realm, `mesail`, holds every person. The access
  token that `identity` issues names one active domain (its `domain`
  claim). The server scopes every message, channel, conversation, search
  and presence query by that domain. A token for one domain never reads or
  changes another domain's data, even when the same person is a member of
  both; such a request gets `404` (not `403`), so the existence of data is
  not revealed.
- **AC-ID-3** Roles come from the token, which `identity` fills: plain
  member for every member of the active domain, `mesail-admin`
  (DomainAdmin) only when the person administers the active domain (data in
  `directory`), `mesail-operator` (Operator, install-wide) from the
  Keycloak realm role. Service calls use a `Service` token. A caller
  without the needed role gets `403` on admin and operator routes. A
  DomainAdmin token of one domain gets `404` on another domain's admin
  routes. The check is on the server; the client only hides buttons.
- **AC-ID-4** A person disabled or signed out in Keycloak loses access to
  every domain within 60 seconds. A person removed from a domain loses
  access to that domain within 60 seconds and keeps their other domains:
  the tab that showed it opens the person's default domain without a
  password. In both cases API calls with the old token get `401` and open
  realtime connections close.
- **AC-ID-5** Mesail stores no password, passkey, TOTP secret or recovery
  code. The browser never talks to Keycloak and never receives a Keycloak
  token. The browser keeps the Mesail access token (60 seconds) in memory
  only, and keeps only an encrypted, opaque session key in per-tab session
  storage, which is gone when the tab closes. No token or key ever appears
  in a URL or a log. (When TACHYON can send same-origin cookies, the
  session key moves to an HttpOnly cookie.) On a kept device (AC-OF-1)
  the session key is kept in the device copy instead, so the app opens
  offline after a restart.
- **AC-ID-6** One sign-in gives a person every domain they are a member
  of. The domain switcher lists those domains; choosing one needs no
  password and changes only the current tab. Another tab keeps its own
  domain. A request made with one domain's token never returns data of
  another domain.
- **AC-ID-7** A person changes their own password in Mesail's settings.
  They must type the current password (and their one-time code when 2-step
  sign-in is on). The new password must meet the install's password
  policy; a refusal says which rule failed. The new password is the
  person's one password, for every domain. After the change, every other
  session of the person ends; this session continues. The browser never
  talks to Keycloak.
- **AC-ID-8** A person turns on 2-step sign-in in Mesail's settings: Mesail
  shows a QR code and the same secret as text, and turns it on only after
  the person types a valid code from their app. Turning it off or
  resetting it needs the current password and a valid code. A lost
  authenticator is reset by an admin (AC-ID-13), not by the person.
- **AC-ID-9** The sign-in form has one field, "Username or email address".
  `ana`, `ANA`, ` ana ` and `ana@example.com` (in any letter case) sign in
  to the same account. With the prefix only, the person lands in their
  default domain (the first domain they were added to, until they choose
  another in settings). With a full address, they land in that domain. An
  unknown prefix, a wrong password and a wrong code get the same `401`
  answer. A correct password with a full address in a domain the person is
  not a member of gets `403 not_a_member` and no session. A correct
  password for a person who is in no domain and is not an Operator gets
  `403 no_domain` and no session. A person who must choose a new password
  sees only the password change screen until they do.
- **AC-ID-10** A prefix belongs to at most one person on the install. Every
  member address of that person is `<prefix>@<domain>`. No alias or
  channel address in any domain may use a person's prefix (`409
  duplicate`), and no new person may take a prefix that an alias or a
  channel address uses in any domain (`409 duplicate`). A deleted person's
  prefix stays reserved for ever (Q5).
- **AC-ID-11** Prefix and address rules. A prefix has 1 to 64 characters
  from `a-z`, `0-9`, `.`, `_` and `-`; it starts and ends with a letter or
  digit and has no two dots in a row. Input in any letter case is accepted
  and stored in lowercase. The role names `postmaster`, `abuse`,
  `hostmaster`, `webmaster`, `mailer-daemon`, `noreply`, `no-reply` and
  `security` are never prefixes; a domain may use them as aliases. Domain
  names are lowercase (IDNA A-label). Hosted addresses are stored and
  compared in lowercase, so mail to `Ana@Example.com` reaches Ana. An
  external address is sent exactly as written and is compared without
  letter case when Mesail matches participants and replies.
- **AC-ID-12** A DomainAdmin adds a member by prefix. For a new prefix they
  give a display name and a first password, and the person must choose a
  new password at their first sign-in. For a prefix that belongs to an
  existing person, the person is added as they are (no password asked),
  and the admin is told it is an existing person. The person sees the new
  domain at their next sign-in, refresh or switch (within 60 seconds).
  Removing a member ends their access to that domain within 60 seconds and
  makes their address there unknown to senders (`550`); the prefix stays
  theirs, and their messages stay with their display name. A DomainAdmin
  may make a member an admin or remove that role. A domain always keeps at
  least one DomainAdmin (`409 last_admin`).
- **AC-ID-13** Resetting a person's password, removing their 2-step code,
  disabling or deleting the person, or changing their display name affects
  every domain of that person. An Operator may do it. A DomainAdmin may do
  it only when they administer every domain the person belongs to (at
  least one), and never to an Operator; otherwise `403`.
- **AC-ID-14** An Operator is a person with the Keycloak role
  `mesail-operator`, and signs in with the same form. An Operator who is in
  no domain gets an install-wide session (domain `*`) that works only in
  the admin app. Operator routes accept any session of an Operator; they
  never use its active domain as the tenant.

### Channels of a domain (AC-CH)
- **AC-CH-1** A member can create a public channel in their domain, join
  and leave any public channel of the domain, and is invited to private
  channels. A private channel is invisible (not listed, `404` on direct
  access) to non-members. A member can never see, join or be invited to a
  channel of another domain.
- **AC-CH-2** A channel name is unique in its domain, 1 to 80 characters,
  lowercase letters, digits, `-` and `_`.
- **AC-CH-3** An archived channel is read-only and still searchable. A
  DomainAdmin or the channel creator can archive it.
- **AC-CH-4** A DomainAdmin can give a channel an email address in the
  channel's own domain. The address must not already belong to a member,
  alias or other channel.

### Messages and threads (AC-MS)
- **AC-MS-1** A message posted to a conversation is visible to every
  participant, and only to them. Online participants receive it over the
  realtime connection within 2 seconds (p95, same region). Chat
  participants are always members of the conversation's domain.
- **AC-MS-2** A reply carries `threadRootId`. The root shows the reply count
  and the last reply time. A reply to a reply attaches to the same root
  (threads are one level deep). The author may tick "Also send to
  #channel" (or "Also send to the conversation"); the reply then also
  appears in the conversation, marked as a thread reply with a link to its
  root.
- **AC-MS-3** The author can edit a message (the message shows "edited")
  and delete it (the body is removed, a "deleted" placeholder stays). Email
  deliveries already sent are not recalled; the UI says so before delete.
- **AC-MS-4** Chat message bodies are Markdown (CommonMark subset:
  emphasis, links, code, lists, quotes). The server stores the source and
  sends the client typed blocks made from it (CONTRACTS.md "Rich text");
  the client never renders HTML from a message. Links are only `https`,
  `http` and `mailto`, and open with `rel="noopener noreferrer"`.
- **AC-MS-5** Attachments up to the domain limit (default 25 MiB per
  file, 50 MiB per message, at most 10 files per message) can be added to
  any message. They are stored
  once and shared by chat and email deliveries.
- **AC-MS-6** A member can add and remove emoji reactions from a fixed set
  (12 in the first release). Each reaction shows its glyph and has an
  accessible name. Reactions are not sent by email and never cross a
  domain.
- **AC-MS-7** Each member has a read position per conversation. Unread
  counts and mention counts in the sidebar match it.
- **AC-MS-8** Sending is idempotent: the client sends a `clientMessageId`,
  and a retry with the same id never creates a second message or email.

### One composer for chat and email (AC-CP)
- **AC-CP-1** For each recipient the composer resolves the address before
  sending (`POST /api/v1/directory/recipients/resolve`) and shows the
  result on the chip. The resolver returns one kind per recipient:
  `member` and `channel` (same domain) show **Chat**; `otherDomain` shows
  **Email · other domain**; `external` shows **Email**. The resolver never
  reveals more about another domain than "this address is hosted here".
- **AC-CP-2** Defaults and allowed choices per kind:
  `member`: default **Chat**, the user may switch to **Email** ("Send as
  email") or **Both**. `channel`: **Chat** only. `otherDomain`: **Email**
  only; the Chat option is visible, disabled, and gives the reason "Chat
  stays inside <sender's domain>" (for example "Chat stays inside
  example.com"). `external`: **Email** only; the Chat option is visible,
  disabled, and gives the reason "<address> has no Mesail account".
- **AC-CP-3** When at least one delivery is **Email**, the composer shows
  Subject (required for a new conversation, prefilled `Re: ...` for a
  reply), Cc and Bcc. Cc and Bcc recipients are resolved like To
  recipients. Bcc recipients are never revealed to other recipients, in
  chat or in email headers.
- **AC-CP-4** When a message has both chat and email recipients, the
  composer shows a one-line summary of who gets what before send.
- **AC-CP-5** The server re-resolves every recipient at send time, in the
  sender's domain, and refuses a send whose resolution differs from what
  the client showed (`409 recipients_changed`), so the user never sends by
  a channel they did not see. If the client claimed `member` for an
  address that is not a member of the sender's domain, the answer is
  `409 recipients_changed`. If the client asked for chat delivery to a
  recipient of a kind that does not allow chat (`otherDomain` or
  `external`), the answer is `422 chat_not_allowed`. Nothing is sent in
  either case.
- **AC-CP-6** An outbound email has a correct `Message-ID`, `In-Reply-To`
  and `References` so replies thread in the recipient's client, is DKIM
  signed for the sender's domain, and uses the sender's primary address or
  a chosen alias (or the channel address for a channel thread) as From.
- **AC-CP-7** Each delivery has a status visible on the message: chat
  (`delivered`, `read`), email (`queued`, `sent`, `deferred`, `bounced`,
  `failed`). A bounce shows the reason on the message.
- **AC-CP-8** In a channel thread that started from an inbound email, a
  reply goes to the outside sender only when the author ticks "Send to
  <address>". Without it the reply stays inside the domain.

### Inbound email (AC-IN)
- **AC-IN-1** SMTP accepts mail only for hosted addresses (members, aliases,
  channel addresses of any hosted domain) and refuses unknown recipients at
  `RCPT TO` with `550 5.1.1`.
- **AC-IN-2** The server answers `250` to `DATA` only after the raw
  message, its metadata and its routing record are durable.
- **AC-IN-3** An inbound email whose `In-Reply-To` or `References` matches
  a message Mesail sent or received in the same domain joins that
  conversation and thread. Otherwise it starts a new email conversation
  (for a member address) or a new channel message (for a channel address).
  A match in another domain never joins a conversation across domains.
- **AC-IN-4** SPF, DKIM and DMARC results are recorded and shown on the
  message. A DMARC `reject` policy failure is refused at SMTP time.
  Messages that fail other checks are marked as suspicious, not dropped.
- **AC-IN-5** HTML email is converted on the server, through a strict
  allowlist, into the same typed blocks as chat (paragraphs, headings,
  lists, quotes, code, tables as plain rows, links, images). Scripts,
  styles, forms, frames and event attributes are dropped. Remote images are
  shown as a placeholder until the user allows images for that sender. The
  plain-text part is kept, and "View original" downloads the raw message as
  a file; it is never rendered.
- **AC-IN-6** An email sent to two members of the same domain is stored
  once and appears in one shared conversation for both. An email sent to
  members of two different domains is stored once per domain, with one
  conversation in each domain.

### Search (AC-SR)
- **AC-SR-1** Search covers message bodies, email subjects, sender names and
  addresses, and attachment file names, in conversations of the caller's
  domain that the member can read now.
- **AC-SR-2** A member never gets a search hit from a conversation they
  cannot read, or from another domain, even through counts or snippets.
- **AC-SR-3** Filters: conversation, author, date range, has-attachment,
  is-email. Results are newest first and paged by cursor.

### Presence and notifications (AC-PN)
- **AC-PN-1** Presence states are `active`, `idle` (no input for 10
  minutes), `dnd` (set by the user, with an end time) and `offline` (no
  connection for 60 seconds). Presence is visible only inside the member's
  domain. A presence query for a member of another domain gets `404`.
- **AC-PN-2** A mention, a DM, or a reply in a thread the member takes part
  in creates a notification. Muted conversations create notifications only
  for direct mentions.
- **AC-PN-3** Notifications reach the member inside Mesail (the
  notification list, and a system notification while a Mesail tab is open
  but not focused) and, in a browser, also when no Mesail tab is open,
  through browser push (decided by the owner on 2026-11-11: "Add browser
  push"):
  - Browser push is off until the member turns it on: in the settings
    ("Notify me in this browser when Mesail is closed"), or from a one-time
    in-app suggestion. The browser's own permission prompt appears only
    after that click. The setting says to turn it on only on the member's
    own device. Mesail never asks on page load. "Not now", or closing the
    suggestion, hides it for good in that browser; the settings row stays.
  - It works in every domain of the person that this browser has opened
    while push was on; each domain registers the browser once.
  - A push shows who and where, never message text: for example "Ana
    Example mentioned you in #design (example.com)". A click opens that
    conversation in that domain at the first unread message, switching the
    tab's domain if needed.
  - Nothing is pushed during `dnd` or while the member is `active` in
    Mesail.
  - Signing out, a wipe of the device copy, removal from a domain and
    `403 access_revoked` stop the pushes: sign-out and wipes end the
    browser's subscription; removal deletes that domain's registration.
  - Where the browser has no push (the installed apps, and Safari on an
    iPhone or iPad unless Mesail was added to the Home Screen), the setting
    is hidden; on an iPhone or iPad, one hint line in its place explains
    that adding Mesail to the Home Screen allows notifications. Push in the installed apps (APNs, FCM, WNS) comes after the
    first release (TACHYON T4b).
  - The email summary (AC-PN-4) still covers anything not read.
  - Needs a TACHYON release with its push client (TACHYON ADR 0118, after
    26.46.02).
- **AC-PN-4** A member may opt in to one email summary of unread
  notifications after a delay they choose (15 minutes to 24 hours).

### Domains and mail operations (AC-DM)
- **AC-DM-1** An Operator can add, verify and remove domains. Adding a
  domain creates its empty messaging space; it does not change Keycloak.
  Removing a domain removes every membership in it; the people and their
  prefixes stay. The verification shows MX, SPF, DKIM and DMARC records
  and their state. A DomainAdmin can see the verification state of their
  own domain only.
- **AC-DM-2** Each domain has its own DKIM key pair. The private key is
  held by the platform secret store, never in FYLO documents or logs. Keys
  can be rotated without lost mail.
- **AC-DM-3** Outbound mail goes through a delivery queue with retries
  (exponential backoff, up to 5 days by default), bounce handling and a
  per-domain suppression list. One domain's suppression list never affects
  another's.
- **AC-DM-4** An outbound item leaves the queue only after a final outcome
  is recorded (sent, bounced, expired).
- **AC-DM-5** A chat message from one hosted domain to another is refused.
  The server answers `409 recipients_changed` or `422 chat_not_allowed`
  (AC-CP-5). No chat delivery, conversation, notification or presence data
  is created in the other domain.
- **AC-DM-6** An email from one hosted domain to another is delivered. The
  install may hand it over internally without sending it over the
  internet, but it is still email: it has full headers, is DKIM signed for
  the sender's domain and checked like inbound mail (AC-IN-4), and it
  appears as an email conversation on both sides. A reply comes back the
  same way and joins the sender's email conversation (AC-IN-3).

### Offline first (AC-OF)
The server stays the source of truth (ADR 0006). A device keeps a copy of
recent data, works offline, and syncs when it is back online.

- **AC-OF-1 Device copy only on a kept device.** At sign-in the person
  chooses "Keep me signed in on this device" (on by default in the
  installed apps, off by default in a browser, Q8). Only then does Mesail
  keep a device copy and a long-lived session on that device. Without it,
  nothing of the person stays on the device after the tab or app closes
  (AC-ID-5 as before).
- **AC-OF-2 Recent window.** For each domain of the person, the device copy
  holds the conversations the person can read (channels, direct
  conversations, email conversations), with every message of the last 30
  days on a computer and 14 days on a phone (the person may choose 7, 14,
  30 or 90), and at least the last 20 messages of each conversation; the
  thread replies of those messages; the person's read positions and
  reactions; the domain's member list (names and addresses) and channel
  list. Older messages and attachments are fetched when the person opens
  them, and are kept only while space allows (AC-OF-13). The first
  offline release waits for TACHYON's sealed tables (Q16), so it already
  has this full window and attachments.
- **AC-OF-3 Reading offline.** With no network, the person can open every
  conversation, thread, message and attachment in the device copy, and
  their drafts. The interface says "Offline" and when the copy was last
  updated. Anything not in the copy says "Not on this device" instead of
  failing silently.
- **AC-OF-4 Writing offline.** With no network, the person can send a
  message or an email (new or reply), edit and delete their own messages,
  add and remove reactions, and move their read position. Each change is
  queued on the device, shown at once with "Waiting to send", and sent
  automatically when the device is online, in the order it was made for
  each conversation. Retrying never sends anything twice (AC-MS-8).
  Files can be attached offline too (up to the domain's limit per file):
  the file is kept with the queued item, uploaded first when the device is
  online, and a refused upload sends the item to "Needs your attention".
  Queued files are never removed to save space; they count against the
  budget, and at most 200 MiB of queued files are kept per device (more
  says "Attach this when you are online"). Creating, joining or leaving a
  channel, admin actions and settings need the network.
- **AC-OF-5 Recipients offline.** The composer resolves names and
  addresses from the device copy of the domain's member and channel
  lists. Any other address shows the chip "Checked when online". When the
  queued item is sent, the server checks every recipient again
  (AC-CP-5); if a kind changed, the item is not sent and goes to "Needs
  your attention" with the reason.
- **AC-OF-6 Needs your attention.** A queued item that the server refuses
  (recipients changed, chat not allowed, too large, the conversation is
  gone or no longer readable, an attachment refused) is never dropped
  silently and never sent with a changed choice. It stays on the device
  with the reason and the actions Edit, Retry and Discard. Other items
  continue. Edits and reactions that depend on a refused send are dropped
  with it, and the person is told.
- **AC-OF-7 Catch-up.** Within 15 seconds of being online again (for up
  to 1,000 changes), the device shows every change made elsewhere while it
  was offline: new messages and replies, edits, deletions, reactions, read
  positions moved on the person's other devices, conversations and
  channels added or removed. Messages are ordered by the time the server
  received them; a message written offline also shows "Written offline at
  <time>".
- **AC-OF-8 Conflicts.**
  - An offline edit of a message that was edited again elsewhere (by the
    same author on another device) is refused (`409 edit_conflict`); the
    device shows the newer text and keeps the offline edit as a draft the
    person may apply again (Q12).
  - A deletion wins over any edit or reaction; an offline edit or
    reaction on a deleted message is dropped, and the person is told.
  - Reactions are a set per person: adding twice or removing a missing
    reaction changes nothing.
  - A read position only moves forward: an older position from an
    offline device never moves it back.
- **AC-OF-9 Access lost.** When the device is next online and finds that
  the person was removed from a domain, it deletes that domain's copy and
  queued items. When the person is disabled or deleted, or an admin ended
  their sessions with "Sign out on all devices", it deletes every copy of
  the person. Before deleting queued items it tells the person once how
  many were not sent, with the first words of each (Q13).
- **AC-OF-10 Sign-out and another person.** Signing out deletes every
  device copy of the person, queued items and drafts included; when
  queued items exist the person is warned first. When another person
  signs in on the device, every copy of the previous person is deleted
  first. One person never sees another person's copy.
- **AC-OF-11 Domains.** Each domain of the person has its own copy, sync
  position and queue. Switching domains works offline between domains
  that have a copy. A queued item is sent only with a token for its own
  domain.
- **AC-OF-12 Search offline.** Offline, search runs on the device copy
  with the same query rules (AC-SR) and marks its results "On this
  device". Online, search uses the server.
- **AC-OF-13 Space.** The device copy stays within a budget (default 1 GiB
  on a computer and 300 MiB on a phone, attachments included; Q9). When it
  is full, Mesail removes, in this order: attachments and messages
  fetched on demand, attachments in the window (least recently opened
  first), messages older than the window. It never removes queued items
  or drafts. Settings show the space used per domain and the free space
  the device reports, let the person choose the budget (300 MB, 1 GB, 2 GB
  or 5 GB; never more than the device reports free), and offer "Remove the
  copy on this device". Removing the copy also turns off "Keep me signed
  in on this device": the session then lasts only until Mesail closes, and
  no copy comes back until the person ticks the box at a later sign-in. When the device refuses a write because its
  storage is full, Mesail stops adding to the copy, keeps the queue and
  drafts, and says "This device is full" (ADR 0006 section 9).
- **AC-OF-14 At rest.** The copy holds no password and no Keycloak token,
  and the stored session key is sealed by the server. After 30 days
  without a successful sync, the copy is locked until the person signs in
  online again (Q10). Every table and file of the copy is encrypted with a
  device key that service code never sees (TACHYON's sealed tables, ADR
  0006 section 9): a non-extractable browser key on the web, and the
  system's key store in the apps (Keychain, Android Keystore, Windows
  DPAPI, the Linux Secret Service). Only ids stay readable on disk
  (person, domain, conversation and message ids). A wipe makes the copy
  unreadable at once. No device keeps a copy before this encryption exists
  (Q16).
- **AC-OF-18 No encryption on this device.** Where the copy cannot be
  encrypted (Linux without a running Secret Service, or Mesail opened over
  plain `http`), Mesail still keeps the copy, unencrypted (Q15), and says
  so: next to "Keep me signed in on this device" at sign-in, the warning
  "This device cannot encrypt Mesail's copy. Anyone who can read this
  device's files can read your recent messages." and the same warning in
  the space settings, with "Remove the copy on this device". The person
  may untick the box to keep nothing. The copy follows every other rule
  (wipes, the 30-day lock, the budget). When the device can encrypt later,
  Mesail deletes the unencrypted copy and builds an encrypted one.
- **AC-OF-15 Long offline.** When a device was offline longer than the
  server keeps changes (30 days), each conversation's copy is reloaded
  from its recent window instead of synced; queued items are kept and sent
  as usual.
- **AC-OF-16 Several windows.** Windows and tabs of the same app on one
  device share the copy and the queue. A queued item is accepted by the
  server once, whichever tab sends it. (Until TACHYON request T11 of ADR
  0006, another open tab shows changes of the copy only after its next
  sync or reload.)
- **AC-OF-17 Email offline.** Email conversations follow the same rules.
  A queued email is checked on the device for what the device can check
  (a subject for a new email conversation, attachment sizes); a later
  bounce is a delivery status (AC-DM-3), not a queue failure. A mention
  of someone who cannot read the conversation (for example, they left the
  channel) is not a refusal: the message is sent, the mention shows as
  plain text, and that person gets no notification (AC-PN-2).
- **AC-OF-19 A tab that is not kept.** When the network drops in a tab
  without "Keep me signed in on this device", the tab keeps showing what
  it already loaded and lets the person write. What they write waits in
  the tab's memory only, shown as "Waiting to send. Closing this tab
  loses it.", and is sent when the network is back while the tab is open.
  Nothing is stored on the device (AC-OF-1). Search, files not yet loaded
  and other conversations say "Needs the network on this browser"; files
  cannot be attached offline in such a tab.

### Interface quality (AC-UI)
- **AC-UI-1** Layout: a domain switcher at the top, a sidebar (channels,
  DMs, email), a conversation pane, and an optional thread pane. Works from
  360 px wide to desktop; on narrow screens one pane at a time.
- **AC-UI-2** WCAG 2.2 AA: keyboard access to every action, visible focus,
  labelled controls, live-region announcements for new messages, contrast.
- **AC-UI-3** Every route stays under the project's per-route transfer
  budget for scripts and WebAssembly.
- **AC-UI-4** Loading, empty and error states exist for every list and
  pane. No raw error text is shown to the user.

### Operations (AC-OP)
- **AC-OP-1** Each backend service and each frontend feature can be built
  and deployed on its own (see ADR 0004), and has a health route.
- **AC-OP-2** No secret, token, message body or attachment content appears
  in logs, traces or metrics.
- **AC-OP-3** Every list, body, attachment, queue and retry has an explicit
  limit, documented in the configuration reference.
- **AC-OP-4 On-premise.** Every feature runs on the owner's own machines
  with no cloud provider account, no managed cloud service and no SaaS,
  at run time and at deploy time (ADR 0007).
- **AC-OP-5 No hidden outbound traffic.** At run time Mesail connects only
  to hosts named in its settings: the DNS resolver, the mail relay or
  (when turned on) recipients' mail servers, an S3-compatible store (when
  chosen), and the push services (Q19). No such setting has a public or
  cloud host as its default.
- **AC-OP-6 Nothing downloaded at run time.** A deployed install starts
  and runs with no download of images, binaries, modules, fonts or
  scripts.
- **AC-OP-7 Operator-provided basics.** TLS certificates are files from
  any CA the operator chooses (an internal CA works); time comes from the
  operator's NTP server; secrets are files on the host. Mesail needs no
  public CA, ACME service or public time server of its own.
- **AC-OP-8 Proof.** With all outbound traffic blocked except the
  configured resolver, relay and (when on) push services, the whole stack
  starts and every blackbox test passes, except those that need external
  mail or push; no other outbound attempt happens.

## 6. Out of scope for the first release

- Voice and video calls, screen sharing.
- Chat between domains of any kind: shared channels, cross-domain DMs or
  shared presence (Slack Connect style). Domains talk to each other by
  email only. This is a product rule, not only a first-release limit.
- A separate account or password per domain. One person has one login for
  all their domains (ADR 0005).
- Renaming a username (prefix). It is fixed when the person is created.
- Sub-addressing (`ana+news@example.com`) in the first release (Q6,
  decided).
- Bots and app integrations beyond incoming webhooks.
- IMAP and POP access for third-party mail clients. (SMTP submission for
  third-party clients is a later slice; see SLICES.md.)
- External accounts (Gmail, Microsoft, IMAP sync), calendar and contacts
  sync. The Go code for them stays in `internal/` and is ported after the
  first release.
- In-browser translation models. The UI is localizable from the start.
- End-to-end encryption.
- Migrating CADUCEUS user credentials. Users enrol again in the Keycloak
  realm `mesail`; mail data is migrated by a separate tool.

## 7. Open questions

- **Q2** Should **Both** (chat and email to a `member`) be offered, or only
  **Chat** or **Email**? This spec offers it (AC-CP-2).
- **Q3** Should the internal hand-over between two hosted domains
  (AC-DM-6) still pass through the outbound queue and the suppression list
  of the sender's domain? This spec says yes, so it behaves like any other
  email.

Decided by the owner on 2026-11-11 (one login, ADR 0005):

- **Q4 (decided: the admin adds).** A DomainAdmin adds an existing person
  (whose prefix is taken) to their domain without that person agreeing.
  The admin learns that the prefix exists, and the person simply gets the
  new domain (AC-ID-12). No invitation step.
- **Q5 (decided: never).** A deleted person's prefix is never given to
  another person, so mail meant for the old person never reaches a new one
  (AC-ID-10). There is no tool to release it.
- **Q6 (decided: not in the first release).** Sub-addressing is not
  supported: `+` is not allowed in a prefix or an alias, and an address
  such as `ana+news@example.com` is unknown (`550`).
- **Q7 (decided: the default domain).** A sign-in with the prefix only
  opens the person's default domain: the first domain they were added to,
  until they choose another in their settings (AC-ID-9).

Decided by the owner on 2026-11-11 (offline first, ADR 0006; "Accept
all"):

- **Q8 (decided: off in a browser, on in the apps).** "Keep me signed in
  on this device" is off by default in a browser, so a shared computer
  keeps nothing, and on by default in the installed apps (AC-OF-1).
- **Q9 (decided).** The default budget and window are 1 GiB and 30 days on
  a computer, 300 MiB and 14 days on a phone (AC-OF-2, AC-OF-13).
- **Q10 (decided: 30 days).** A device copy that has not reached the
  server for 30 days is locked until an online sign-in (AC-OF-14).
- **Q11 (decided).** On a computer, images and files up to 2 MiB in the
  recent window are downloaded ahead of time; on a phone, only when opened,
  or on Wi-Fi when the person allows it in settings. (Only once
  attachments are kept on devices, ADR 0006 T2.)
- **Q12 (decided: refuse and keep a draft).** An offline edit that meets a
  newer edit is refused (`409 edit_conflict`), and the offline text is
  kept as a draft (AC-OF-8).
- **Q13 (decided: first words).** When access to a domain is lost, the
  one-time notice shows how many queued items were not sent and the first
  words of each (AC-OF-9).
- **Q14 (decided: on the device).** In the first release drafts stay on
  the device; they do not follow the person to other devices.

Decided by the owner on 2026-11-11, after TACHYON's plan (ADR 0006
section 9); both differ from the first proposal:

- **Q15 (decided: an unencrypted copy with a warning).** Where the copy
  cannot be encrypted (Linux without a running Secret Service, or plain
  `http`), Mesail keeps an unencrypted copy and shows a warning
  (AC-OF-18).
- **Q16 (decided: wait for encryption).** The first offline release waits
  until TACHYON's sealed tables (its wave 3) are in a TACHYON release that
  Mesail uses. No device keeps a copy before that (AC-OF-14).

Decided by the owner on 2026-11-11 (on-premise, ADR 0007):

- **Q17 (decided: no cloud).** Everything runs on the owner's machines with
  no cloud account or cloud service. The install may still use the
  internet for email with outside people, DNS lookups and (when turned on)
  browser push. Not air-gapped.
- **Q18 (decided: Lambda images in Floci).** In production on-premise the
  services run as Lambda images in the Floci emulator on kestrel (through
  PaaS, like FitNest), keeping the serverless-first shape. ADR 0007
  section 4 lists the rules that contain Floci (its own VM, an API never
  reachable from outside, no route out, pinned local images).
- **Q19 (decided: optional, off by default).** Browser push is a feature
  an Operator turns on (`notify.push.enabled`) when the install may reach
  the browser makers' push services.
- **Q20 (decided: builds may download).** Builds may download pinned,
  checksum-checked parts from the internet; a deployed install never
  downloads anything (AC-OP-6). No offline build bundle.

Q1 (one shared conversation or one per member) is closed: one email to two
members of the same domain is one shared conversation (AC-IN-6).
