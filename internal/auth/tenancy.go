// Package auth owns the CADUCEUS authentication and authorization boundary.
//
// Every credential, session, token, and permission decision belongs to one
// SESAME engine this process owns. No other runtime speaks to that engine, and
// no CADUCEUS feature may reach persistence with an identity this package did
// not decide. The legacy runtime reaches these decisions only through the
// authenticated loopback bridge.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// TenantName is the one SESAME tenant this deployment owns.
//
// A mail domain is not a tenant. SESAME group membership requires the
// principal and the group to share a tenant, so a domain-per-tenant model
// would make one person administering two domains two separate identities with
// two separate logins. Domains are resources instead — `mailbox:<domain>:*`
// and `domain:<domain>` — and authority over one is a group membership. A
// person joins as many domain groups as they administer and still signs in
// once.
//
// The isolation this gives up is SESAME's tenant boundary between mail
// domains. What enforces separation instead is unchanged and is where it was
// already enforced: every decision is default-deny over a domain-qualified
// resource, and FYLO's per-domain UID/GID ownership sits below any
// caller-controlled filter.
const TenantName = "caduceus"

// Names in SESAME are a bounded slug: lowercase letters, digits, and interior
// hyphens, at most 63 bytes. A mail domain is none of those things reliably,
// and a mailbox local part may be 64 bytes on its own, so neither is ever used
// as a name directly.
const (
	domainDigestLength  = 8
	domainSlugLimit     = 30
	mailboxDigestLength = 12
)

// domainLabel is the bounded, deterministic label for one mail domain inside a
// name. The digest suffix keeps distinct domains that share a slug apart, which
// plain punctuation substitution does not: "a-b.com" and "a.b-com" both slug to
// "a-b-com". The readable prefix is a courtesy for operators reading a ledger.
func domainLabel(domain string) (string, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(normalized))
	slug := strings.Trim(slugify(normalized), "-")
	if len(slug) > domainSlugLimit {
		slug = strings.Trim(slug[:domainSlugLimit], "-")
	}
	if slug == "" {
		// A domain of nothing but punctuation cannot reach here: it fails
		// NormalizeDomain. The guard keeps the invariant local anyway.
		return "", fmt.Errorf("domain %q has no usable label", domain)
	}
	return slug + "-" + hex.EncodeToString(digest[:])[:domainDigestLength], nil
}

// DomainAdminRole and DomainViewerRole name one domain's roles.
func DomainAdminRole(domain string) (string, error) {
	label, err := domainLabel(domain)
	return "domain-admin-" + label, err
}

// DomainViewerRole names one domain's read-only role.
func DomainViewerRole(domain string) (string, error) {
	label, err := domainLabel(domain)
	return "domain-viewer-" + label, err
}

// DomainAdminGroup and DomainViewerGroup name the groups those roles are
// granted to. Membership, not a per-principal grant, is what carries domain
// authority: it is one operation to add or remove, and it is what lets one
// principal hold authority over several domains at once.
func DomainAdminGroup(domain string) (string, error) {
	label, err := domainLabel(domain)
	return "domain-admins-" + label, err
}

// DomainViewerGroup names one domain's read-only group.
func DomainViewerGroup(domain string) (string, error) {
	label, err := domainLabel(domain)
	return "domain-viewers-" + label, err
}

// MailboxRoleName is the name of the role covering exactly one mailbox.
//
// It is a digest rather than the address: a local part may be 64 bytes and a
// domain 253, and a name may be 63. Role names are not the audit surface —
// resources are, and `mailbox:corp.com:chi` stays legible in every decision
// trace and ledger entry.
func MailboxRoleName(address Address) string {
	digest := sha256.Sum256([]byte(address.String()))
	return "mailbox-" + hex.EncodeToString(digest[:])[:mailboxDigestLength]
}

func slugify(value string) string {
	var builder strings.Builder
	previousHyphen := false
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			builder.WriteRune(character)
			previousHyphen = false
		default:
			if !previousHyphen {
				builder.WriteByte('-')
				previousHyphen = true
			}
		}
	}
	return builder.String()
}

// NormalizeDomain lowercases and validates one mail domain.
func NormalizeDomain(domain string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	if normalized == "" {
		return "", errors.New("domain is required")
	}
	if len(normalized) > 253 {
		return "", errors.New("domain must not exceed 253 bytes")
	}
	labels := strings.Split(normalized, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain %q must contain at least two labels", domain)
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return "", fmt.Errorf("domain %q has an invalid label", domain)
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("domain %q has a label bounded by a hyphen", domain)
		}
		for _, character := range label {
			switch {
			case character >= 'a' && character <= 'z':
			case character >= '0' && character <= '9':
			case character == '-':
			default:
				return "", fmt.Errorf("domain %q contains unsupported character %q", domain, character)
			}
		}
	}
	return normalized, nil
}

// Address is one normalized mailbox address split into the parts the
// authorization vocabulary needs.
type Address struct {
	Local  string
	Domain string
}

// String returns the normalized address.
func (address Address) String() string {
	return address.Local + "@" + address.Domain
}

// ParseAddress normalizes one mailbox address and enforces the local-part
// shape SESAME resource patterns accept.
//
// The constraint is deliberate rather than encoded around. A resource that
// reads `mailbox:del.ma:chi` is legible in the security ledger and in an
// operator's decision trace; an escaped one is not. Plus-addressing is a
// delivery-time concept applied to a mailbox that already exists, so it is
// unaffected by the shape of the mailbox's own name.
func ParseAddress(address string) (Address, error) {
	normalized := strings.ToLower(strings.TrimSpace(address))
	at := strings.LastIndex(normalized, "@")
	if at <= 0 || at == len(normalized)-1 {
		return Address{}, fmt.Errorf("address %q must be local@domain", address)
	}
	local := normalized[:at]
	domain, err := NormalizeDomain(normalized[at+1:])
	if err != nil {
		return Address{}, err
	}
	if len(local) > 64 {
		return Address{}, fmt.Errorf("address %q has a local part over 64 bytes", address)
	}
	for _, character := range local {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= '0' && character <= '9':
		case character == '-' || character == '_' || character == '.':
		default:
			return Address{}, fmt.Errorf(
				"mailbox %q may contain only letters, digits, and the characters -_. before the @",
				address,
			)
		}
	}
	return Address{Local: local, Domain: domain}, nil
}

// Authorization vocabulary. Actions and resources are colon-joined lowercase
// segments; only a permission may end in a wildcard segment.
const (
	// ActionMailboxRead covers reading messages, folders, threads, search,
	// attachments, and any other retrieval from one mailbox.
	ActionMailboxRead = "mailbox:read"
	// ActionMailboxWrite covers mutating message state: read/unread, star,
	// move, archive, trash, snooze, drafts, rules, templates, signatures.
	ActionMailboxWrite = "mailbox:write"
	// ActionMailboxSend covers submitting and scheduling outbound mail.
	ActionMailboxSend = "mailbox:send"
	// ActionDomainRead covers reading one domain's configuration.
	ActionDomainRead = "domain:read"
	// ActionDomainAdmin covers changing one domain's configuration, its
	// mailboxes, aliases, routes, and suppression list.
	ActionDomainAdmin = "domain:admin"
	// ActionCredentialManage covers enrolling or removing a principal's own
	// authenticators. It requires a recently proven factor, not only a grant.
	ActionCredentialManage = "credential:manage"
)

// MailboxResource returns the resource pattern for one mailbox.
func MailboxResource(address Address) string {
	return "mailbox:" + address.Domain + ":" + address.Local
}

// DomainResource returns the resource pattern for one domain's configuration.
func DomainResource(domain string) string {
	return "domain:" + domain
}

// DomainMailboxes returns the permission pattern covering every mailbox in one
// domain. Only a role permission may carry it; a decision request may not.
func DomainMailboxes(domain string) string {
	return "mailbox:" + domain + ":*"
}
