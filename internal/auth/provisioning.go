package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/d31ma/sesame/clients/go/sesame"
)

// Stable engine error codes this package reacts to rather than reports.
const (
	codeIdentifierConflict = "identifier_conflict"
	codeRoleExists         = "role_exists"
	codeGrantExists        = "grant_exists"
	codeGroupExists        = "group_exists"
	codeGroupMemberExists  = "group_member_exists"
	codePrincipalNotFound  = "principal_not_found"
	codeTenantNotFound     = "tenant_not_found"
)

// The two kinds of authority one mailbox may hold over a mail domain. Each is
// a group; holding it is membership.
const (
	// RoleDomainAdmin may change a domain's configuration and act on every
	// mailbox in it.
	RoleDomainAdmin = "domain-admin"
	// RoleDomainViewer may read a domain's configuration and mailboxes.
	RoleDomainViewer = "domain-viewer"
)

// Provisioning is the result of provisioning one mailbox.
type Provisioning struct {
	PrincipalID string `json:"principalId"`
	TenantID    string `json:"tenantId"`
	RoleID      string `json:"roleId,omitempty"`
	GrantID     string `json:"grantId,omitempty"`
	Address     string `json:"address"`
	Domain      string `json:"domain"`
	Created     bool   `json:"created"`
}

// DomainProvisioning is one provisioned mail domain: the roles that describe
// authority over it and the groups that carry it.
type DomainProvisioning struct {
	Domain        string `json:"domain"`
	TenantID      string `json:"tenantId"`
	AdminRoleID   string `json:"adminRoleId"`
	ViewerRoleID  string `json:"viewerRoleId"`
	AdminGroupID  string `json:"adminGroupId"`
	ViewerGroupID string `json:"viewerGroupId"`
}

// ProvisionDomain creates one mail domain's roles and the groups they are
// granted to. It is safe to repeat.
func (engine *Engine) ProvisionDomain(ctx context.Context, domain string) (DomainProvisioning, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return DomainProvisioning{}, err
	}
	tenantID, err := engine.TenantID(ctx)
	if err != nil {
		return DomainProvisioning{}, err
	}

	adminRoleName, _ := DomainAdminRole(normalized)
	adminRoleID, err := engine.ensureRole(ctx, tenantID, adminRoleName, []sesame.Permission{
		{Action: ActionDomainAdmin, Resource: DomainResource(normalized)},
		{Action: ActionDomainRead, Resource: DomainResource(normalized)},
		{Action: ActionMailboxRead, Resource: DomainMailboxes(normalized)},
		{Action: ActionMailboxWrite, Resource: DomainMailboxes(normalized)},
		{Action: ActionMailboxSend, Resource: DomainMailboxes(normalized)},
	})
	if err != nil {
		return DomainProvisioning{}, err
	}
	viewerRoleName, _ := DomainViewerRole(normalized)
	viewerRoleID, err := engine.ensureRole(ctx, tenantID, viewerRoleName, []sesame.Permission{
		{Action: ActionDomainRead, Resource: DomainResource(normalized)},
		{Action: ActionMailboxRead, Resource: DomainMailboxes(normalized)},
	})
	if err != nil {
		return DomainProvisioning{}, err
	}

	// The role is granted to the group once, here. After this, giving or
	// taking away domain authority is a membership change and touches no
	// grants at all.
	adminGroupName, _ := DomainAdminGroup(normalized)
	adminGroupID, err := engine.ensureGroup(ctx, tenantID, adminGroupName, adminRoleID)
	if err != nil {
		return DomainProvisioning{}, err
	}
	viewerGroupName, _ := DomainViewerGroup(normalized)
	viewerGroupID, err := engine.ensureGroup(ctx, tenantID, viewerGroupName, viewerRoleID)
	if err != nil {
		return DomainProvisioning{}, err
	}

	return DomainProvisioning{
		Domain:        normalized,
		TenantID:      tenantID,
		AdminRoleID:   adminRoleID,
		ViewerRoleID:  viewerRoleID,
		AdminGroupID:  adminGroupID,
		ViewerGroupID: viewerGroupID,
	}, nil
}

// ProvisionMailbox creates one mailbox principal and the role and grant that
// give it authority over its own mailbox and nothing else.
func (engine *Engine) ProvisionMailbox(ctx context.Context, mailbox string) (Provisioning, error) {
	address, err := ParseAddress(mailbox)
	if err != nil {
		return Provisioning{}, err
	}
	tenantID, err := engine.TenantID(ctx)
	if err != nil {
		return Provisioning{}, err
	}
	principal, created, err := engine.ensurePrincipal(ctx, tenantID, address)
	if err != nil {
		return Provisioning{}, err
	}
	resource := MailboxResource(address)
	roleID, err := engine.ensureRole(ctx, tenantID, MailboxRoleName(address), []sesame.Permission{
		{Action: ActionMailboxRead, Resource: resource},
		{Action: ActionMailboxWrite, Resource: resource},
		{Action: ActionMailboxSend, Resource: resource},
	})
	if err != nil {
		return Provisioning{}, err
	}
	grantID, err := engine.ensureGrant(ctx, tenantID, principal.ID, roleID)
	if err != nil {
		return Provisioning{}, err
	}
	return Provisioning{
		PrincipalID: principal.ID,
		TenantID:    tenantID,
		RoleID:      roleID,
		GrantID:     grantID,
		Address:     address.String(),
		Domain:      address.Domain,
		Created:     created,
	}, nil
}

// Membership is one mailbox's authority over one mail domain.
type Membership struct {
	Address string `json:"address"`
	Domain  string `json:"domain"`
	Role    string `json:"role"`
	GroupID string `json:"groupId"`
	Added   bool   `json:"added"`
}

// GrantDomainRole gives one mailbox authority over one mail domain, which need
// not be its own.
//
// This is what makes multi-domain administration one identity: the mailbox
// joins the domain's group, and it may be a member of as many domain groups as
// it administers while still signing in once.
func (engine *Engine) GrantDomainRole(ctx context.Context, mailbox, domain, role string) (Membership, error) {
	group, address, normalized, err := engine.domainGroup(ctx, mailbox, domain, role)
	if err != nil {
		return Membership{}, err
	}
	principal, err := engine.principalByAddress(ctx, group.TenantID, address)
	if err != nil {
		return Membership{}, err
	}
	groupID := group.AdminGroupID
	if role == RoleDomainViewer {
		groupID = group.ViewerGroupID
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	added := true
	if err := engine.client.GroupMemberAdd(callCtx, groupID, principal.ID); err != nil {
		if !hasCode(err, codeGroupMemberExists) {
			return Membership{}, fmt.Errorf("add %s to the %s group for %s: %w", mailbox, role, normalized, err)
		}
		added = false
	}
	return Membership{
		Address: address.String(),
		Domain:  normalized,
		Role:    role,
		GroupID: groupID,
		Added:   added,
	}, nil
}

// RevokeDomainRole takes one mailbox's authority over one mail domain away.
// Its authority over any other domain is untouched.
func (engine *Engine) RevokeDomainRole(ctx context.Context, mailbox, domain, role string) (Membership, error) {
	group, address, normalized, err := engine.domainGroup(ctx, mailbox, domain, role)
	if err != nil {
		return Membership{}, err
	}
	principal, err := engine.principalByAddress(ctx, group.TenantID, address)
	if err != nil {
		return Membership{}, err
	}
	groupID := group.AdminGroupID
	if role == RoleDomainViewer {
		groupID = group.ViewerGroupID
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	if err := engine.client.GroupMemberRemove(callCtx, groupID, principal.ID); err != nil {
		return Membership{}, fmt.Errorf(
			"remove %s from the %s group for %s: %w", mailbox, role, normalized, err,
		)
	}
	return Membership{
		Address: address.String(),
		Domain:  normalized,
		Role:    role,
		GroupID: groupID,
	}, nil
}

func (engine *Engine) domainGroup(
	ctx context.Context,
	mailbox, domain, role string,
) (DomainProvisioning, Address, string, error) {
	if role != RoleDomainAdmin && role != RoleDomainViewer {
		return DomainProvisioning{}, Address{}, "", fmt.Errorf(
			"role %q must be %s or %s", role, RoleDomainAdmin, RoleDomainViewer,
		)
	}
	address, err := ParseAddress(mailbox)
	if err != nil {
		return DomainProvisioning{}, Address{}, "", err
	}
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return DomainProvisioning{}, Address{}, "", err
	}
	provisioned, err := engine.ProvisionDomain(ctx, normalized)
	if err != nil {
		return DomainProvisioning{}, Address{}, "", err
	}
	return provisioned, address, normalized, nil
}

// SuspendMailbox stops one mailbox principal from authenticating. Existing
// sessions stop being honoured because a suspended principal's grants no
// longer stand.
func (engine *Engine) SuspendMailbox(ctx context.Context, mailbox string) error {
	address, err := ParseAddress(mailbox)
	if err != nil {
		return err
	}
	tenantID, err := engine.TenantID(ctx)
	if err != nil {
		return err
	}
	principal, err := engine.principalByAddress(ctx, tenantID, address)
	if err != nil {
		return err
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	if _, err := engine.client.PrincipalSuspend(callCtx, principal.ID); err != nil {
		return fmt.Errorf("suspend mailbox %s: %w", address, err)
	}
	return nil
}

func (engine *Engine) ensurePrincipal(
	ctx context.Context,
	tenantID string,
	address Address,
) (sesame.Principal, bool, error) {
	callCtx, cancel := engine.call(ctx)
	principal, err := engine.client.PrincipalCreate(callCtx, tenantID, "human", sesame.PrincipalIdentifier{
		Namespace: identifierNamespace,
		Value:     address.String(),
	})
	cancel()
	if err == nil {
		return principal, true, nil
	}
	if !hasCode(err, codeIdentifierConflict) {
		return sesame.Principal{}, false, fmt.Errorf("create mailbox principal %s: %w", address, err)
	}
	existing, err := engine.principalByAddress(ctx, tenantID, address)
	if err != nil {
		return sesame.Principal{}, false, err
	}
	return existing, false, nil
}

func (engine *Engine) principalByAddress(
	ctx context.Context,
	tenantID string,
	address Address,
) (sesame.Principal, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	principal, err := engine.client.PrincipalGetByIdentifier(callCtx, tenantID, sesame.PrincipalIdentifier{
		Namespace: identifierNamespace,
		Value:     address.String(),
	})
	if err != nil {
		return sesame.Principal{}, fmt.Errorf("resolve mailbox principal %s: %w", address, err)
	}
	return principal, nil
}

// ensureRole creates one role and records its identifier, or returns the
// identifier recorded when it was created. SESAME has no operation to read a
// role back by name, so the deployment registry is what makes provisioning
// repeatable across restarts.
func (engine *Engine) ensureRole(
	ctx context.Context,
	tenantID, name string,
	permissions []sesame.Permission,
) (string, error) {
	if roleID, found := engine.recallRole(tenantID, name); found {
		return roleID, nil
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	role, err := engine.client.RoleCreate(callCtx, tenantID, name, permissions)
	if err == nil {
		if err := engine.rememberRole(tenantID, name, role.ID); err != nil {
			return "", err
		}
		return role.ID, nil
	}
	if hasCode(err, codeRoleExists) {
		return "", engine.lostRegistryError("role", name, tenantID)
	}
	return "", fmt.Errorf("create role %s: %w", name, err)
}

// ensureGroup creates one group, grants it its role, and records it.
func (engine *Engine) ensureGroup(ctx context.Context, tenantID, name, roleID string) (string, error) {
	if groupID, found := engine.recallGroup(tenantID, name); found {
		return groupID, nil
	}
	callCtx, cancel := engine.call(ctx)
	group, err := engine.client.GroupCreate(callCtx, tenantID, name)
	cancel()
	if err != nil {
		if hasCode(err, codeGroupExists) {
			return "", engine.lostRegistryError("group", name, tenantID)
		}
		return "", fmt.Errorf("create group %s: %w", name, err)
	}
	grantCtx, cancelGrant := engine.call(ctx)
	_, err = engine.client.GrantCreateForGroup(grantCtx, tenantID, group.ID, roleID)
	cancelGrant()
	if err != nil && !hasCode(err, codeGrantExists) {
		return "", fmt.Errorf("grant role to group %s: %w", name, err)
	}
	if err := engine.rememberGroup(tenantID, name, group.ID); err != nil {
		return "", err
	}
	return group.ID, nil
}

// lostRegistryError reports the one situation the registry cannot recover
// from: the object exists in the engine, but the file that recorded its
// identifier is gone, and the machine protocol will not read it back by name.
func (engine *Engine) lostRegistryError(kind, name, tenantID string) error {
	return fmt.Errorf(
		"%s %s exists in tenant %s but is absent from the provisioning registry at %s; "+
			"restore that file to use it again",
		kind, name, tenantID, engine.options.RegistryFile,
	)
}

func (engine *Engine) ensureGrant(ctx context.Context, tenantID, principalID, roleID string) (string, error) {
	if roleID == "" {
		return "", errors.New("a grant requires a role identifier")
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	grant, err := engine.client.GrantCreate(callCtx, tenantID, principalID, roleID)
	if err == nil {
		return grant.ID, nil
	}
	if hasCode(err, codeGrantExists) {
		return "", nil
	}
	return "", fmt.Errorf("grant role: %w", err)
}

// hasCode reports whether an error is one specific engine protocol error.
func hasCode(err error, code string) bool {
	var protocol *sesame.ProtocolError
	if errors.As(err, &protocol) {
		return protocol.Code == code
	}
	// The SDK wraps some transport failures without the typed error. Matching
	// the code text is a fallback, not the contract.
	return strings.Contains(err.Error(), code)
}
