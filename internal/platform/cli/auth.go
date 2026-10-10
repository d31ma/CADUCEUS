package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/auth"
	"github.com/d31ma/caduceus/internal/platform/config"
	"github.com/d31ma/caduceus/internal/platform/preflight"
)

// runAuth implements the operator commands for the identity deployment. They
// are deliberately separate from `serve` and `fylo-bridge`: creating the key
// boundary and provisioning tenants are deliberate acts, not startup side
// effects.
func runAuth(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeAuthUsage(stdout)
		return ExitSuccess
	}
	switch args[0] {
	case "init":
		return runAuthInit(ctx, args[1:], stdout, stderr)
	case "status":
		return runAuthEngine(ctx, args[1:], stdout, stderr, "status", nil)
	case "domain":
		return runAuthProvision(ctx, args[1:], stdout, stderr, "domain")
	case "mailbox":
		return runAuthProvision(ctx, args[1:], stdout, stderr, "mailbox")
	case "role":
		return runAuthProvision(ctx, args[1:], stdout, stderr, "role")
	case "unrole":
		return runAuthProvision(ctx, args[1:], stdout, stderr, "unrole")
	case "suspend":
		return runAuthProvision(ctx, args[1:], stdout, stderr, "suspend")
	case "credentials":
		return runAuthProvision(ctx, args[1:], stdout, stderr, "credentials")
	case "help", "-h", "--help":
		writeAuthUsage(stdout)
		return ExitSuccess
	default:
		_, _ = fmt.Fprintf(stderr, "caduceus auth: unknown action %q\n\n", args[0])
		writeAuthUsage(stderr)
		return ExitUsage
	}
}

// runAuthInit creates the deployment the engine runs against. It refuses to
// touch an existing one: the signing key inside it is what every issued token
// was verified against, and replacing it silently would end every session with
// no explanation.
func runAuthInit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("auth init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus auth init: unexpected positional arguments")
		return ExitUsage
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus auth init: %v\n", err)
		return ExitFailure
	}
	if _, err := os.Stat(filepath.Join(cfg.SESAMEDeployment, "config.json")); err == nil {
		_, _ = fmt.Fprintf(
			stdout,
			"SESAME deployment %s already exists; nothing to do.\n",
			cfg.SESAMEDeployment,
		)
		return ExitSuccess
	}
	if cfg.SESAMEIssuer == "" {
		_, _ = fmt.Fprintln(stderr, "caduceus auth init: SESAME_ISSUER is required")
		return ExitFailure
	}
	initContext, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	command := exec.CommandContext(
		initContext,
		cfg.SESAMEBinary,
		"init",
		"--deployment", cfg.SESAMEDeployment,
		"--fylo-binary", cfg.FYLOBinary,
		"--issuer", cfg.SESAMEIssuer,
	)
	command.Stderr = stderr
	output, err := command.Output()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus auth init: %v\n", err)
		return ExitFailure
	}
	_, _ = stdout.Write(output)
	return ExitSuccess
}

func runAuthProvision(ctx context.Context, args []string, stdout, stderr io.Writer, action string) int {
	flags := flag.NewFlagSet("auth "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	domain := flags.String("domain", "", "mail domain")
	address := flags.String("address", "", "mailbox address")
	role := flags.String("role", "", "domain-admin or domain-viewer")
	password := flags.String("password", "", "initial password for the mailbox owner")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "caduceus auth %s: unexpected positional arguments\n", action)
		return ExitUsage
	}
	request := map[string]string{}
	switch action {
	case "domain":
		if *domain == "" {
			_, _ = fmt.Fprintln(stderr, "caduceus auth domain: --domain is required")
			return ExitUsage
		}
		request["domain"] = *domain
	case "mailbox", "suspend", "credentials":
		if *address == "" {
			_, _ = fmt.Fprintf(stderr, "caduceus auth %s: --address is required\n", action)
			return ExitUsage
		}
		request["address"] = *address
	case "role", "unrole":
		// The domain is explicit: a mailbox may administer a domain that is
		// not its own, which is the whole point of a domain group.
		if *address == "" || *role == "" || *domain == "" {
			_, _ = fmt.Fprintf(
				stderr, "caduceus auth %s: --address, --domain, and --role are required\n", action,
			)
			return ExitUsage
		}
		request["address"] = *address
		request["domain"] = *domain
		request["role"] = *role
	}
	if action == "credentials" {
		if *password == "" {
			_, _ = fmt.Fprintln(stderr, "caduceus auth credentials: --password is required")
			return ExitUsage
		}
		request["password"] = *password
	}
	return runAuthEngine(ctx, []string{"-config", *configPath}, stdout, stderr, action, request)
}

// runAuthEngine starts the engine, performs one operation, and stops it. The
// operator commands do not share the long-running owner: two processes writing
// one FYLO root is exactly what the deployment forbids, so a provisioning
// command is run while the service is stopped, or through its bridge.
func runAuthEngine(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	action string,
	request map[string]string,
) int {
	flags := flag.NewFlagSet("auth "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus auth %s: %v\n", action, err)
		return ExitFailure
	}
	report, err := preflight.RunPersistence(ctx, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus auth %s: %v\n", action, err)
		return ExitFailure
	}
	engine, err := startAuthEngine(ctx, cfg, report, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus auth %s: %v\n", action, err)
		return ExitFailure
	}
	defer engine.Close()

	result, err := performAuthAction(ctx, engine, action, request)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus auth %s: %v\n", action, err)
		return ExitFailure
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus auth %s: encode output: %v\n", action, err)
		return ExitFailure
	}
	return ExitSuccess
}

func performAuthAction(
	ctx context.Context,
	engine *auth.Engine,
	action string,
	request map[string]string,
) (any, error) {
	switch action {
	case "status":
		return engine.Dispatch(ctx, "auth.status", json.RawMessage(`{}`))
	case "domain":
		return engine.ProvisionDomain(ctx, request["domain"])
	case "mailbox":
		return engine.ProvisionMailbox(ctx, request["address"])
	case "role":
		return engine.GrantDomainRole(ctx, request["address"], request["domain"], request["role"])
	case "unrole":
		return engine.RevokeDomainRole(ctx, request["address"], request["domain"], request["role"])
	case "suspend":
		if err := engine.SuspendMailbox(ctx, request["address"]); err != nil {
			return nil, err
		}
		return map[string]bool{"suspended": true}, nil
	case "credentials":
		// The recovery codes are the mailbox owner's first second factor and
		// are printed once. They are how a new owner reaches MFA assurance and
		// can then enrol a passkey of their own.
		if err := engine.SetInitialPassword(ctx, request["address"], request["password"]); err != nil {
			return nil, err
		}
		codes, err := engine.IssueInitialRecoveryCodes(ctx, request["address"])
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"address":       request["address"],
			"passwordSet":   true,
			"recoveryCodes": codes.Codes,
			"note":          "Give these to the mailbox owner. They are shown once and each code is single use.",
		}, nil
	default:
		return nil, errors.New("unknown auth action " + action)
	}
}

func writeAuthUsage(output io.Writer) {
	_, _ = fmt.Fprintln(output, strings.TrimSpace(`
Usage: caduceus auth <action> [options]

Actions:
  init                       Create the SESAME deployment and its key boundary
  status                     Report engine readiness, release, and policy
  domain --domain D          Provision one mail domain's tenant and roles
  mailbox --address A        Provision one mailbox principal, role, and grant
  role --address A --domain D --role R
                             Join a mailbox to a domain's admin or viewer group
  unrole --address A --domain D --role R
                             Remove a mailbox from that group
  credentials --address A --password P
                             Set the first password and issue recovery codes
  suspend --address A        Stop one mailbox from authenticating`))
}
