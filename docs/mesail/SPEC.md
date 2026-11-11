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
- Hosting: the services run as AWS Lambda container images (Floci locally,
  AWS or kestrel later). Details are in ADR 0004.

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
| Recipient kind | The result of resolving one recipient address for one sender. Exact API values: `member` (a member or alias of the sender's domain), `channel` (a channel address of the sender's domain), `otherDomain` (a member, alias or channel address of another domain hosted by this install), `external` (any other address). |
| Channel | A named conversation in a domain. Channels belong to the domain directly. Public (any member of the domain may join) or private (invite only). A channel may have an email address in its domain, so mail can be sent into it. |
| Direct conversation | A conversation between a fixed set of participants, outside any channel. With one other participant it is a direct message (DM); with more it is a group DM. Chat participants are members of the same domain. Other participants take part by email. |
| Conversation | A channel or a direct conversation. Every conversation belongs to exactly one domain. Every message belongs to exactly one conversation. |
| Message | One post in a conversation. It has an author, a body, optional attachments, and, when email was involved, email headers. |
| Thread | The replies to one message (the thread root). Replies stay in the same conversation. |
| Delivery | How one recipient receives one message: `chat` or `email`. Each delivery has its own status. |
| Email conversation | A direct conversation that has at least one participant who takes part by email (an `otherDomain` or `external` address, or a member switched to email). It is still a conversation; email is how those participants take part. |
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
away, Bo gets a push notification. If he does not read it within the time
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
  access to that domain within 60 seconds and keeps their other domains.
  In both cases API calls get `401` and open realtime connections close.
- **AC-ID-5** Mesail stores no password, passkey, TOTP secret or recovery
  code. The browser never talks to Keycloak and never receives a Keycloak
  token. The browser keeps the Mesail access token (60 seconds) in memory
  only, and keeps only an encrypted, opaque session key in per-tab session
  storage, which is gone when the tab closes. No token or key ever appears
  in a URL or a log. (When TACHYON can send same-origin cookies, the
  session key moves to an HttpOnly cookie.)
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
  prefix stays reserved (Q5).
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
  it only when they administer every domain the person belongs to;
  otherwise `403`.
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
- **AC-PN-3** In the first release, notifications reach the member inside
  Mesail: in the notification list, and as a system notification
  (browser or desktop notification) while a Mesail window is open but not
  focused. None are shown during `dnd`. When no Mesail window is open, the
  email summary (AC-PN-4) covers them. Push to a closed browser or to a
  phone waits for TACHYON support (Web Push from Go Tac) and native push per
  platform, both after the first release.
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

## 6. Out of scope for the first release

- Voice and video calls, screen sharing.
- Chat between domains of any kind: shared channels, cross-domain DMs or
  shared presence (Slack Connect style). Domains talk to each other by
  email only. This is a product rule, not only a first-release limit.
- A separate account or password per domain. One person has one login for
  all their domains (ADR 0005).
- Renaming a username (prefix). It is fixed when the person is created.
- Sub-addressing (`ana+news@example.com`) in the first release (Q6).
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

- **Q4** May a DomainAdmin add an existing person (whose prefix is taken)
  to their domain without that person agreeing? This spec says yes (the
  owner's answer): the admin learns that the prefix exists, and the person
  simply gets the new domain. If two unrelated organizations may share one
  install, the alternative is an invitation the person accepts, and the
  shared prefix namespace itself becomes a problem (two organizations
  cannot both have an `info` person).
- **Q5** May a deleted person's prefix ever be given to a new person? This
  spec says no: it stays reserved, so mail meant for the old person never
  reaches a new one. A later Operator tool may release a prefix by hand.
- **Q6** Should `ana+anything@domain` reach Ana (sub-addressing)? This spec
  says not in the first release: `+` is not allowed in a prefix or alias,
  and such an address is unknown.
- **Q7** Which domain opens after a sign-in with the prefix only? This spec
  says the person's default domain: the first domain they were added to,
  until they choose another in their settings. The alternative is the last
  domain used, which needs stored state per person.

Q1 (one shared conversation or one per member) is closed: one email to two
members of the same domain is one shared conversation (AC-IN-6).
