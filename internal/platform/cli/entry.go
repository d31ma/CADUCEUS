package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/d31ma/caduceus/internal/mail/inbound"
	"github.com/d31ma/caduceus/internal/mail/mailauth"
	"github.com/d31ma/caduceus/internal/runtime/entry"
)

// runEntry is the container entry point that replaced scripts/entry.mjs.
func runEntry(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	entry.InboundAuthenticator = mailauth.ForInbound()
	return entry.Run(ctx, args, stdout, stderr, entry.Options{})
}

// runInboundSMTP serves only the inbound MX until the context ends. It reads
// the same settings as the entry point's inbound-smtp service and delivers to
// CADUCEUS_API_URL (default http://127.0.0.1:$PORT).
func runInboundSMTP(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus inbound-smtp: unexpected arguments")
		return ExitUsage
	}
	options, err := inbound.FromSettings("")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus inbound-smtp: %v\n", err)
		return ExitFailure
	}
	options.Stdout, options.Stderr, options.Authenticate = stdout, stderr, mailauth.ForInbound()
	server, err := inbound.Start(options)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus inbound-smtp: %v\n", err)
		return ExitFailure
	}
	select {
	case <-ctx.Done():
	case err := <-server.Errors():
		if err != nil {
			_ = server.Close(context.Background())
			return ExitFailure
		}
	}
	if err := server.Close(context.Background()); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus inbound-smtp: %v\n", err)
		return ExitFailure
	}
	return ExitSuccess
}
