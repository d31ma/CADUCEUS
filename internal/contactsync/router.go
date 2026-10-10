package contactsync

import (
	"context"

	"github.com/d31ma/caduceus/internal/accounts"
)

// Router keeps provider selection out of the contact synchronization job.
// Connectors receive only the exact account and the worker's short-lived
// decrypted credential.
type Router struct {
	connectors map[accounts.Provider]Connector
}

func NewRouter(connectors map[accounts.Provider]Connector) *Router {
	copy := make(map[accounts.Provider]Connector, len(connectors))
	for provider, connector := range connectors {
		if connector != nil {
			copy[provider] = connector
		}
	}
	return &Router{connectors: copy}
}

func (router *Router) connector(provider accounts.Provider) (Connector, error) {
	if router == nil || router.connectors == nil || router.connectors[provider] == nil {
		return nil, accounts.ErrUnsupported
	}
	return router.connectors[provider], nil
}

func (router *Router) Sync(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, state State) (Snapshot, error) {
	connector, err := router.connector(account.Provider)
	if err != nil {
		return Snapshot{}, err
	}
	return connector.Sync(ctx, account, credentials, state)
}

func (router *Router) ApplyMutation(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, mutation Mutation, current Contact) (MutationOutcome, error) {
	connector, err := router.connector(account.Provider)
	if err != nil {
		return "", err
	}
	return connector.ApplyMutation(ctx, account, credentials, mutation, current)
}
