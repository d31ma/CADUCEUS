// Package cli implements the CADUCEUS operator command line.
package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsend"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/adapters/bridgeapi"
	"github.com/d31ma/caduceus/internal/adapters/caldav"
	"github.com/d31ma/caduceus/internal/adapters/carddav"
	"github.com/d31ma/caduceus/internal/adapters/fylostore"
	"github.com/d31ma/caduceus/internal/adapters/googlecalendar"
	"github.com/d31ma/caduceus/internal/adapters/googlecontacts"
	"github.com/d31ma/caduceus/internal/adapters/googleoauth"
	"github.com/d31ma/caduceus/internal/adapters/mailprovider"
	"github.com/d31ma/caduceus/internal/adapters/microsoftcalendar"
	"github.com/d31ma/caduceus/internal/adapters/microsoftcontacts"
	"github.com/d31ma/caduceus/internal/adapters/microsoftoauth"
	"github.com/d31ma/caduceus/internal/adapters/providerdiscovery"
	"github.com/d31ma/caduceus/internal/auth"
	"github.com/d31ma/caduceus/internal/calendarsync"
	"github.com/d31ma/caduceus/internal/contactsync"
	"github.com/d31ma/caduceus/internal/platform/accountids"
	"github.com/d31ma/caduceus/internal/platform/accountsecrets"
	"github.com/d31ma/caduceus/internal/platform/buildinfo"
	"github.com/d31ma/caduceus/internal/platform/config"
	"github.com/d31ma/caduceus/internal/platform/fylo"
	"github.com/d31ma/caduceus/internal/platform/fyloaudit"
	"github.com/d31ma/caduceus/internal/platform/fylobackup"
	"github.com/d31ma/caduceus/internal/platform/fylobridge"
	"github.com/d31ma/caduceus/internal/platform/httpserver"
	"github.com/d31ma/caduceus/internal/platform/preflight"
)

const (
	// ExitSuccess indicates successful command completion.
	ExitSuccess = 0
	// ExitFailure indicates an operational or validation failure.
	ExitFailure = 1
	// ExitUsage indicates invalid command syntax.
	ExitUsage = 2
)

// Run executes one CLI invocation and returns its process exit code.
func Run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	info buildinfo.Info,
) int {
	if len(args) == 0 {
		writeUsage(stdout)
		return ExitSuccess
	}
	switch args[0] {
	case "help", "-h", "--help":
		writeUsage(stdout)
		return ExitSuccess
	case "version":
		return runVersion(args[1:], stdout, stderr, info)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout, stderr)
	case "serve":
		return runServe(ctx, args[1:], stderr, info)
	case "fylo-bridge":
		return runFYLOBridge(ctx, args[1:], stderr, info)
	case "backup":
		return runBackup(ctx, args[1:], stdout, stderr)
	case "attachments":
		return runAttachments(ctx, args[1:], stdout, stderr)
	case "migrate":
		return runMigrate(ctx, args[1:], stdout, stderr)
	case "auth":
		return runAuth(ctx, args[1:], stdout, stderr)
	case "entry":
		return runEntry(ctx, args[1:], stdout, stderr)
	case "inbound-smtp":
		return runInboundSMTP(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "caduceus: unknown command %q\n\n", args[0])
		writeUsage(stderr)
		return ExitUsage
	}
}

func runAttachments(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "reconcile" {
		_, _ = fmt.Fprintln(stderr, "caduceus attachments: expected reconcile")
		return ExitUsage
	}
	flags := flag.NewFlagSet("attachments reconcile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	apply := flags.Bool("apply", false, "delete confirmed orphan bucket objects")
	if err := flags.Parse(args[1:]); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus attachments reconcile: unexpected positional arguments")
		return ExitUsage
	}
	cfg, report, ok := loadPersistence(ctx, *configPath, "attachments reconcile", stderr)
	if !ok {
		return ExitFailure
	}
	owner := fylo.New(fyloOptions(cfg, report))
	if err := owner.Start(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus attachments reconcile: %v\n", err)
		return ExitFailure
	}
	defer owner.Close(context.Background())
	result, err := fyloaudit.Reconcile(ctx, owner, *apply)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus attachments reconcile: %v\n", err)
		return ExitFailure
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus attachments reconcile: encode output: %v\n", err)
		return ExitFailure
	}
	return ExitSuccess
}

func runFYLOBridge(
	ctx context.Context,
	args []string,
	stderr io.Writer,
	info buildinfo.Info,
) int {
	flags := flag.NewFlagSet("fylo-bridge", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: unexpected positional arguments")
		return ExitUsage
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: %v\n", err)
		return ExitFailure
	}
	if len(cfg.FYLOBridgeSecret) < 32 {
		_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: FYLO_BRIDGE_SECRET must contain at least 32 characters")
		return ExitFailure
	}
	activeKey, credentialKeys, err := cfg.ExternalAccountKeyring()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: %v\n", err)
		return ExitFailure
	}
	credentialVault, err := accountsecrets.New(activeKey, credentialKeys)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: external account credential keyring is invalid")
		return ExitFailure
	}
	report, err := preflight.RunPersistence(ctx, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: %v\n", err)
		return ExitFailure
	}
	qualification, err := fylo.QualifyRelease(ctx, fylo.QualificationOptions{
		Binary:               cfg.FYLOBinary,
		ExpectedVersion:      report.FYLOVersion,
		MaxRequestBytes:      cfg.FYLOInteger(cfg.FYLOMaxRequestBytes),
		MaxResponseBytes:     cfg.FYLOInteger(cfg.FYLOMaxResponseBytes),
		MaxAttachmentBytes:   cfg.FYLOInt64(cfg.FYLOMaxAttachmentBytes),
		AttachmentQuotaBytes: cfg.FYLOInt64(cfg.FYLOAttachmentQuota),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: released-runtime qualification: %v\n", err)
		return ExitFailure
	}
	owner := fylo.New(fyloOptions(cfg, report))
	if err := owner.Start(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: %v\n", err)
		return ExitFailure
	}
	defer owner.Close(context.Background())
	if err := fylostore.EnsureProviderCollections(ctx, owner); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: %v\n", err)
		return ExitFailure
	}
	engine, err := startAuthEngine(ctx, cfg, report, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: %v\n", err)
		return ExitFailure
	}
	defer engine.Close()
	accountRepository := fylostore.NewAccounts(owner, cfg.FYLOPosixAccess == "true")
	passwordValidator, err := mailprovider.NewPasswordValidator(
		cfg.ExternalAccountConnectionDuration(),
		cfg.ExternalAccountPrivateCIDRs(),
	)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: external account network policy is invalid")
		return ExitFailure
	}
	cardDAVClient, err := carddav.NewClient(carddav.Options{
		PrivateCIDRs: cfg.ExternalAccountPrivateCIDRs(), Timeout: cfg.ExternalAccountConnectionDuration(),
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: CardDAV network policy is invalid")
		return ExitFailure
	}
	calDAVClient, err := caldav.NewClient(caldav.Options{
		PrivateCIDRs: cfg.ExternalAccountPrivateCIDRs(), Timeout: cfg.ExternalAccountConnectionDuration(),
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: CalDAV network policy is invalid")
		return ExitFailure
	}
	validators := map[accounts.Provider]accounts.Validator{
		accounts.ProviderIMAP: caldav.NewValidator(carddav.NewValidator(passwordValidator, cardDAVClient), calDAVClient),
	}
	calendarConnectors := map[accounts.Provider]calendarsync.Connector{
		accounts.ProviderIMAP: caldav.NewConnector(calDAVClient),
	}
	contactConnectors := map[accounts.Provider]contactsync.Connector{
		accounts.ProviderIMAP: carddav.NewConnector(cardDAVClient),
	}
	var microsoftClient *microsoftoauth.Client
	if cfg.MicrosoftOAuthConfigured() {
		microsoftClient, err = microsoftoauth.New(microsoftoauth.Config{
			ClientID: cfg.MicrosoftOAuthClientID, ClientSecret: cfg.MicrosoftOAuthClientSecret,
			Tenant: cfg.MicrosoftOAuthTenant, RedirectURI: cfg.MicrosoftOAuthRedirectURI,
		})
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: Microsoft OAuth configuration is invalid")
			return ExitFailure
		}
		microsoftCalendarConnector, connectorErr := microsoftcalendar.New(microsoftcalendar.Options{})
		if connectorErr != nil {
			_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: Microsoft Calendar configuration is invalid")
			return ExitFailure
		}
		microsoftContactConnector, connectorErr := microsoftcontacts.New(microsoftcontacts.Options{})
		if connectorErr != nil {
			_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: Microsoft Contacts configuration is invalid")
			return ExitFailure
		}
		validators[accounts.ProviderMicrosoft] = accounts.CapabilityValidator{Base: passwordValidator, Extra: accounts.Capabilities{Calendar: true, Contacts: true}}
		calendarConnectors[accounts.ProviderMicrosoft] = microsoftCalendarConnector
		contactConnectors[accounts.ProviderMicrosoft] = microsoftContactConnector
	}
	var googleClient *googleoauth.Client
	if cfg.GoogleOAuthConfigured() {
		googleClient, err = googleoauth.New(googleoauth.Config{
			ClientID: cfg.GoogleOAuthClientID, ClientSecret: cfg.GoogleOAuthClientSecret, RedirectURI: cfg.GoogleOAuthRedirectURI,
		})
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: Google OAuth configuration is invalid")
			return ExitFailure
		}
		googleCalendarConnector, connectorErr := googlecalendar.New(googlecalendar.Options{})
		if connectorErr != nil {
			_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: Google Calendar configuration is invalid")
			return ExitFailure
		}
		googleContactConnector, connectorErr := googlecontacts.New(googlecontacts.Options{})
		if connectorErr != nil {
			_, _ = fmt.Fprintln(stderr, "caduceus fylo-bridge: Google Contacts configuration is invalid")
			return ExitFailure
		}
		validators[accounts.ProviderGmail] = accounts.CapabilityValidator{Base: passwordValidator, Extra: accounts.Capabilities{Calendar: true, Contacts: true}}
		calendarConnectors[accounts.ProviderGmail] = googleCalendarConnector
		contactConnectors[accounts.ProviderGmail] = googleContactConnector
	}
	accountService := accounts.NewService(
		accountRepository,
		credentialVault,
		validators,
		accounts.ClockFunc(time.Now),
		accountids.New(),
	)
	syncStore, err := fylostore.NewAccountSync(owner, cfg.FYLOPosixAccess == "true", filepath.Join(cfg.FYLOIncomingRoot, "provider-sync"))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: initialize provider sync store: %v\n", err)
		return ExitFailure
	}
	contactJob := contactsync.NewJob(syncStore, contactsync.NewRouter(contactConnectors))
	calendarJob := calendarsync.NewJob(syncStore, calendarsync.NewRouter(calendarConnectors))
	imapConnector, err := mailprovider.NewIMAPConnector(passwordValidator, mailprovider.IMAPSyncOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: initialize IMAP sync connector: %v\n", err)
		return ExitFailure
	}
	smtpConnector, err := mailprovider.NewSMTPConnector(passwordValidator)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: initialize SMTP submission connector: %v\n", err)
		return ExitFailure
	}
	submissionConnectors := map[accounts.Provider]accountsend.Connector{accounts.ProviderIMAP: smtpConnector}
	syncConnectors := map[accounts.Provider]accountsync.Connector{accounts.ProviderIMAP: imapConnector}
	var credentialsSource accountsync.CredentialsSource = accountsync.NewVaultCredentials(accountRepository, credentialVault)
	var microsoftOAuth, googleOAuth bridgeapi.OAuthConnection
	refreshers := map[accounts.Provider]microsoftoauth.TokenRefresher{}
	if microsoftClient != nil {
		submissionConnectors[accounts.ProviderMicrosoft] = smtpConnector
		syncConnectors[accounts.ProviderMicrosoft] = imapConnector
		refreshers[accounts.ProviderMicrosoft] = microsoftClient
		microsoftOAuth = accounts.NewOAuthConnectionService(accountRepository, credentialVault, microsoftClient, accountService, accounts.ClockFunc(time.Now), rand.Reader)
	}
	if googleClient != nil {
		submissionConnectors[accounts.ProviderGmail] = smtpConnector
		syncConnectors[accounts.ProviderGmail] = imapConnector
		refreshers[accounts.ProviderGmail] = googleClient
		googleOAuth = accounts.NewProviderOAuthConnectionService(accountRepository, credentialVault, googleClient, accountService, accounts.OAuthConnectionProfile{
			Provider: accounts.ProviderGmail,
			IMAP:     accounts.Endpoint{Host: "imap.gmail.com", Port: 993, Security: accounts.SecurityTLS},
			SMTP:     accounts.Endpoint{Host: "smtp.gmail.com", Port: 465, Security: accounts.SecurityTLS},
		}, accounts.ClockFunc(time.Now), rand.Reader)
	}
	if len(refreshers) > 0 {
		credentialsSource = microsoftoauth.NewCredentialSourceForProviders(accountRepository, credentialVault, refreshers, time.Now)
	}
	submissionWorker := accountsend.NewWorker(syncStore, submissionConnectors, accountsend.WorkerOptions{})
	syncWorker := accountsync.NewWorker(
		syncStore, accountService, credentialsSource,
		syncConnectors,
		accountsync.WorkerOptions{Holder: fmt.Sprintf("%s-%d", info.Name, os.Getpid()), Jobs: []accountsync.AccountJob{submissionWorker}, SyncJobs: []accountsync.AccountJob{contactJob, calendarJob}},
	)
	syncScheduler := accountsync.NewScheduler(syncStore, accountRepository, syncWorker, accountsync.SchedulerOptions{})
	watchConnectors := map[accounts.Provider]accountsync.ChangeWatcher{accounts.ProviderIMAP: imapConnector}
	if microsoftClient != nil {
		watchConnectors[accounts.ProviderMicrosoft] = imapConnector
	}
	if googleClient != nil {
		watchConnectors[accounts.ProviderGmail] = imapConnector
	}
	syncWatcher := accountsync.NewWatcher(syncStore, syncStore, accountRepository, credentialsSource, watchConnectors, accountsync.WatcherOptions{})
	discovery := accounts.NewDiscovery(
		providerdiscovery.Resolver{},
		providerdiscovery.NewSafeFetcher(nil),
	)
	accountBoundary := bridgeapi.NewAccountsDispatcher(engine, accountService, discovery, bridgeapi.AccountExtensions{
		Schedule: syncStore, Index: syncStore, UnifiedIndex: syncStore, Mutations: syncWorker, Submissions: submissionWorker, SubmissionReader: syncStore, Contacts: syncStore, ContactMutations: syncStore, Calendars: syncStore, CalendarMutations: syncStore, Notifications: syncStore, OAuth: microsoftOAuth, GoogleOAuth: googleOAuth,
	})
	bridge := fylobridge.New(fylobridge.Options{
		Address:              cfg.FYLOBridgeAddress,
		Secret:               cfg.FYLOBridgeSecret,
		MaxFrameBytes:        fylobridge.ComposedMaxFrameBytes,
		RequestTimeout:       cfg.FYLORequestDuration(),
		MaxAttachmentBytes:   cfg.FYLOInt64(cfg.FYLOMaxAttachmentBytes),
		AttachmentQuotaBytes: cfg.FYLOInt64(cfg.FYLOAttachmentQuota),
		IncomingRoot:         cfg.FYLOIncomingRoot,
		DataRoot:             cfg.FYLORoot,
	}, owner).
		WithAuthorizer(engine, auth.Operations()).
		WithAccounts(accountBoundary, bridgeapi.AccountOperations())
	listener, err := bridge.Listen()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus fylo-bridge: %v\n", err)
		return ExitFailure
	}
	logger := slog.New(slog.NewJSONHandler(stderr, nil)).With(
		"service", info.Name, "version", info.Version, "component", "fylo_bridge",
	)
	logger.Info(
		"FYLO bridge started",
		"event", "fylo_bridge_started",
		"address", listener.Addr().String(),
		"fylo_version", report.FYLOVersion,
		"fylo_target", report.FYLOTarget,
		"sesame_version", engine.Info().Version,
		"auth_operations", len(auth.Operations()),
		"qualification_checks", len(qualification.Checks),
	)
	type componentResult struct {
		name string
		err  error
	}
	runtimeCtx, cancelRuntime := context.WithCancel(ctx)
	results := make(chan componentResult, 3)
	go func() { results <- componentResult{name: "provider_sync", err: syncScheduler.Run(runtimeCtx)} }()
	go func() { results <- componentResult{name: "provider_idle", err: syncWatcher.Run(runtimeCtx)} }()
	go func() { results <- componentResult{name: "fylo_bridge", err: bridge.Serve(runtimeCtx, listener)} }()
	first := <-results
	cancelRuntime()
	failure := first
	for remaining := 0; remaining < 2; remaining++ {
		candidate := <-results
		if failure.err == nil || errors.Is(failure.err, context.Canceled) {
			failure = candidate
		}
	}
	err = failure.err
	if errors.Is(err, context.Canceled) {
		err = nil
	}
	logger.Info("FYLO bridge stopped", "event", "fylo_bridge_stopped")
	if err != nil {
		logger.Error("FYLO bridge failed", "event", "fylo_bridge_failed", "component", failure.name, "error", err.Error())
		return ExitFailure
	}
	return ExitSuccess
}

// startAuthEngine launches the one SESAME process this runtime owns.
func startAuthEngine(
	ctx context.Context,
	cfg config.Config,
	report preflight.Report,
	stderr io.Writer,
) (*auth.Engine, error) {
	redirect := cfg.SESAMERedirect()
	if redirect == "" {
		return nil, errors.New("SESAME_REDIRECT_URI or PUBLIC_URL is required to register the first-party client")
	}
	return auth.Start(ctx, auth.Options{
		Binary:          cfg.SESAMEBinary,
		Deployment:      cfg.SESAMEDeployment,
		ExpectedVersion: report.SESAMEVersion,
		Issuer:          cfg.SESAMEIssuer,
		RedirectURI:     redirect,
		Policy:          cfg.AuthPolicy,
		SessionLifetime: cfg.SESAMESessionDuration(),
		RequestTimeout:  cfg.SESAMERequestDuration(),
		// Config validation already forbids test routes in production, so this
		// cannot be true on a production runtime.
		AllowTestOperations: cfg.EnableTestRoutes == "true",
		Stderr:              stderr,
	})
}

func runBackup(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus backup: expected create, verify, or restore")
		return ExitUsage
	}
	switch args[0] {
	case "create":
		return runBackupCreate(ctx, args[1:], stdout, stderr)
	case "verify":
		return runBackupVerify(ctx, args[1:], stdout, stderr)
	case "restore":
		return runBackupRestore(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "caduceus backup: unknown action %q\n", args[0])
		return ExitUsage
	}
}

func runBackupCreate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup create", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	destination := flags.String("destination", "", "new local backup directory")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if *destination == "" || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus backup create: --destination is required")
		return ExitUsage
	}
	cfg, report, ok := loadPersistence(ctx, *configPath, "backup create", stderr)
	if !ok {
		return ExitFailure
	}
	owner := fylo.New(fyloOptions(cfg, report))
	if err := owner.Start(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus backup create: %v\n", err)
		return ExitFailure
	}
	defer owner.Close(context.Background())
	manifest, err := fylobackup.Create(ctx, owner, *destination)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus backup create: %v\n", err)
		return ExitFailure
	}
	return encodeBackupResult(stdout, stderr, "created", *destination, manifest)
}

func runBackupVerify(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", "", "local backup directory")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if *source == "" || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus backup verify: --source is required")
		return ExitUsage
	}
	manifest, err := fylobackup.Verify(ctx, *source)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus backup verify: %v\n", err)
		return ExitFailure
	}
	return encodeBackupResult(stdout, stderr, "verified", *source, manifest)
}

func runBackupRestore(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup restore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	source := flags.String("source", "", "verified local backup directory")
	destination := flags.String("destination", "", "new restore directory")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if *source == "" || *destination == "" || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus backup restore: --source and --destination are required")
		return ExitUsage
	}
	cfg, report, ok := loadPersistence(ctx, *configPath, "backup restore", stderr)
	if !ok {
		return ExitFailure
	}
	root, _ := filepath.Abs(cfg.FYLORoot)
	target, _ := filepath.Abs(*destination)
	if root == target {
		_, _ = fmt.Fprintln(stderr, "caduceus backup restore: active FYLO root cannot be overwritten")
		return ExitFailure
	}
	manifest, err := fylobackup.Restore(ctx, *source, target)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus backup restore: %v\n", err)
		return ExitFailure
	}
	restoredConfig := cfg
	restoredConfig.FYLORoot = target
	owner := fylo.New(fyloOptions(restoredConfig, report))
	if err := owner.Start(ctx); err != nil {
		_ = os.RemoveAll(target)
		_, _ = fmt.Fprintf(stderr, "caduceus backup restore: restored root failed FYLO validation: %v\n", err)
		return ExitFailure
	}
	if err := owner.Close(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus backup restore: close validation owner: %v\n", err)
		return ExitFailure
	}
	return encodeBackupResult(stdout, stderr, "restored", target, manifest)
}

func loadPersistence(
	ctx context.Context,
	configPath, command string,
	stderr io.Writer,
) (config.Config, preflight.Report, bool) {
	cfg, err := config.Load(configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus %s: %v\n", command, err)
		return config.Config{}, preflight.Report{}, false
	}
	report, err := preflight.RunPersistence(ctx, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus %s: %v\n", command, err)
		return config.Config{}, preflight.Report{}, false
	}
	return cfg, report, true
}

func fyloOptions(cfg config.Config, report preflight.Report) fylo.Options {
	return fylo.Options{
		Binary:               cfg.FYLOBinary,
		Root:                 cfg.FYLORoot,
		ExpectedVersion:      report.FYLOVersion,
		ExpectedDependencies: report.FYLODependencies,
		ExpectedCommit:       report.FYLOCommit,
		ExpectedTarget:       report.FYLOTarget,
		RequiredCapabilities: report.FYLOCapabilities,
		MaxRequestBytes:      cfg.FYLOInteger(cfg.FYLOMaxRequestBytes),
		MaxResponseBytes:     cfg.FYLOInteger(cfg.FYLOMaxResponseBytes),
		RequestTimeout:       cfg.FYLORequestDuration(),
		ShutdownTimeout:      cfg.ShutdownDuration(),
		RestartLimit:         cfg.FYLOInteger(cfg.FYLORestartLimit),
	}
}

func encodeBackupResult(
	stdout, stderr io.Writer,
	status, path string,
	manifest fylobackup.Manifest,
) int {
	result := map[string]any{
		"status": status, "path": path, "entries": len(manifest.Entries),
		"digest": manifest.SourceDigest, "createdAt": manifest.CreatedAt,
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus backup: encode output: %v\n", err)
		return ExitFailure
	}
	return ExitSuccess
}

func runVersion(args []string, stdout, stderr io.Writer, info buildinfo.Info) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus version: unexpected positional arguments")
		return ExitUsage
	}
	switch *output {
	case "text":
		_, _ = fmt.Fprintf(
			stdout,
			"%s %s (commit %s, built %s, %s, %s/%s)\n",
			info.Name,
			info.Version,
			info.Commit,
			info.BuiltAt,
			info.GoVersion,
			info.OS,
			info.Arch,
		)
	case "json":
		if err := json.NewEncoder(stdout).Encode(info); err != nil {
			_, _ = fmt.Fprintf(stderr, "caduceus version: encode output: %v\n", err)
			return ExitFailure
		}
	default:
		_, _ = fmt.Fprintf(stderr, "caduceus version: unsupported output format %q\n", *output)
		return ExitUsage
	}
	return ExitSuccess
}

func runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	output := flags.String("output", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus doctor: unexpected positional arguments")
		return ExitUsage
	}
	cfg, report, ok := loadAndCheck(ctx, *configPath, "doctor", stderr)
	_ = cfg
	if !ok {
		return ExitFailure
	}
	switch *output {
	case "text":
		_, _ = fmt.Fprintf(
			stdout,
			"CADUCEUS runtime is ready (%d frontend files, %d schemas, FYLO %s)\n",
			report.FrontendFiles,
			report.SchemaFiles,
			report.FYLOVersion,
		)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			_, _ = fmt.Fprintf(stderr, "caduceus doctor: encode output: %v\n", err)
			return ExitFailure
		}
	default:
		_, _ = fmt.Fprintf(stderr, "caduceus doctor: unsupported output format %q\n", *output)
		return ExitUsage
	}
	return ExitSuccess
}

func runServe(ctx context.Context, args []string, stderr io.Writer, info buildinfo.Info) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus serve: unexpected positional arguments")
		return ExitUsage
	}
	cfg, report, ok := loadAndCheck(ctx, *configPath, "serve", stderr)
	if !ok {
		return ExitFailure
	}
	logger := slog.New(slog.NewJSONHandler(stderr, nil)).With(
		"service", info.Name,
		"version", info.Version,
		"component", "runtime",
	)
	if err := httpserver.New(cfg, report, info, logger).Run(ctx); err != nil {
		logger.Error("server failed", "event", "server_failed", "error", err.Error())
		return ExitFailure
	}
	return ExitSuccess
}

type migrationResult struct {
	Mode        string `json:"mode"`
	Mutated     bool   `json:"mutated"`
	SchemaFiles int    `json:"schemaFiles"`
	FYLOVersion string `json:"fyloVersion"`
}

func runMigrate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "strict JSON configuration file")
	output := flags.String("output", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "caduceus migrate: unexpected positional arguments")
		return ExitUsage
	}
	_, report, ok := loadAndCheck(ctx, *configPath, "migrate", stderr)
	if !ok {
		return ExitFailure
	}
	result := migrationResult{
		Mode:        "preflight",
		Mutated:     false,
		SchemaFiles: report.SchemaFiles,
		FYLOVersion: report.FYLOVersion,
	}
	switch *output {
	case "text":
		_, _ = fmt.Fprintf(
			stdout,
			"Migration preflight passed (%d schemas, FYLO %s); no Phase 1 data mutation performed.\n",
			result.SchemaFiles,
			result.FYLOVersion,
		)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			_, _ = fmt.Fprintf(stderr, "caduceus migrate: encode output: %v\n", err)
			return ExitFailure
		}
	default:
		_, _ = fmt.Fprintf(stderr, "caduceus migrate: unsupported output format %q\n", *output)
		return ExitUsage
	}
	return ExitSuccess
}

func loadAndCheck(
	ctx context.Context,
	configPath string,
	command string,
	stderr io.Writer,
) (config.Config, preflight.Report, bool) {
	cfg, err := config.Load(configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus %s: %v\n", command, err)
		return config.Config{}, preflight.Report{}, false
	}
	report, err := preflight.Run(ctx, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "caduceus %s: %v\n", command, err)
		return config.Config{}, preflight.Report{}, false
	}
	return cfg, report, true
}

func writeUsage(output io.Writer) {
	_, _ = fmt.Fprintln(output, `Usage: caduceus <command> [options]

Commands:
  version   Print immutable build metadata
  doctor    Validate configuration and runtime dependencies
  serve     Serve the frontend, system API, and operations endpoints
  fylo-bridge  Own FYLO and serve the authenticated legacy bridge
  backup    Create, verify, or restore an offline local FYLO backup
  attachments  Audit or explicitly repair attachment bucket drift
  auth      Initialize the identity deployment and provision domains and mailboxes
  entry     Container entry point: serve (default), admin:create, domain:migrate, help
  inbound-smtp  Run only the inbound SMTP server (MX) from the settings
  migrate   Run the read-only Phase 1 migration preflight
  help      Show this help`)
}
