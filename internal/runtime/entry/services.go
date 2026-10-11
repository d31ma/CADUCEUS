package entry

import (
	"context"
	"time"

	"github.com/d31ma/caduceus/internal/mail/inbound"
	"github.com/d31ma/caduceus/internal/mail/submission"
	"github.com/d31ma/caduceus/internal/runtime/acmecert"
	"github.com/d31ma/caduceus/internal/runtime/settings"
	"github.com/d31ma/caduceus/internal/runtime/supervisor"
	"github.com/d31ma/caduceus/internal/runtime/tickworker"
)

// InboundAuthenticator verifies SPF/DKIM/DMARC for the inbound MX. The CLI
// sets it to the mailauth verifier; nil delivers unverified.
var InboundAuthenticator inbound.Authenticator

func milliseconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Millisecond))
}

func mailServerEnabled() bool { return env("MAIL_SERVER_ENABLED") == "true" }

// productionServices registers the background services entry.mjs started,
// in its order.
func productionServices(session *Session) []supervisor.Service {
	var services []supervisor.Service
	stdout, stderr, apiURL := session.Stdout(), session.Stderr(), session.APIURL()

	if env("MAIL_ACME_ENABLED") == "true" {
		services = append(services, supervisor.Service{
			Name:     "acme-renewal",
			Optional: true,
			Start: func(ctx context.Context) (supervisor.StopFunc, error) {
				dir := env("MAIL_TLS_DIR")
				if dir == "" {
					dir = "/data/tls"
				}
				setDefault("MAIL_TLS_CERT_FILE", dir+"/"+env("MAIL_HOSTNAME")+".cert.pem")
				setDefault("MAIL_TLS_KEY_FILE", dir+"/"+env("MAIL_HOSTNAME")+".key.pem")
				accountPath := env("MAIL_ACME_ACCOUNT_FILE")
				if accountPath == "" {
					accountPath = dir + "/acme-account.pem"
				}
				httpPort := 80
				if value := env("MAIL_ACME_HTTP_PORT"); value != "" {
					if parsed, ok := settings.Number(value); ok {
						httpPort = int(parsed)
					}
				}
				service, err := acmecert.New(acmecert.Options{
					Hostname: env("MAIL_HOSTNAME"), Email: env("MAIL_ACME_EMAIL"),
					CertPath: env("MAIL_TLS_CERT_FILE"), KeyPath: env("MAIL_TLS_KEY_FILE"), AccountPath: accountPath,
					HTTPPort: httpPort, Staging: env("MAIL_ACME_STAGING") == "true", Stdout: stdout, Stderr: stderr,
				})
				if err != nil {
					return nil, err
				}
				if err := service.StartRenewal(ctx); err != nil {
					return nil, err
				}
				return func(context.Context) error { service.Stop(); return nil }, nil
			},
		})
	}

	if mailServerEnabled() {
		services = append(services, supervisor.Service{
			Name: "inbound-smtp",
			Start: func(context.Context) (supervisor.StopFunc, error) {
				options, err := inbound.FromSettings(apiURL)
				if err != nil {
					return nil, err
				}
				options.Stdout, options.Stderr, options.Authenticate = stdout, stderr, InboundAuthenticator
				server, err := inbound.Start(options)
				if err != nil {
					return nil, err
				}
				go reportListenerFailure(session, "inbound-smtp", server.Errors())
				return server.Close, nil
			},
		})
	}

	if env("MAIL_SUBMISSION_ENABLED") == "true" {
		services = append(services, supervisor.Service{
			Name: "smtp-submission",
			Start: func(ctx context.Context) (supervisor.StopFunc, error) {
				var servers []*submission.Server
				closeAll := func(ctx context.Context) error {
					for index := len(servers) - 1; index >= 0; index-- {
						_ = servers[index].Close(ctx)
					}
					return nil
				}
				port := 587
				if value := env("SMTP_SUBMISSION_PORT"); value != "" {
					parsed, _ := settings.Number(value)
					port = int(parsed)
				}
				plain := submission.FromSettings(port, false, apiURL)
				plain.Stdout, plain.Stderr = stdout, stderr
				server, err := submission.Start(plain)
				if err != nil {
					return nil, err
				}
				servers = append(servers, server)
				if value := env("SMTP_SUBMISSION_TLS_PORT"); value != "" {
					parsed, _ := settings.Number(value)
					secure := submission.FromSettings(int(parsed), true, apiURL)
					secure.Stdout, secure.Stderr = stdout, stderr
					server, err := submission.Start(secure)
					if err != nil {
						_ = closeAll(ctx)
						return nil, err
					}
					servers = append(servers, server)
				}
				for _, server := range servers {
					go reportListenerFailure(session, "smtp-submission", server.Errors())
				}
				return closeAll, nil
			},
		})
	}

	secret := env("INBOUND_WEBHOOK_SECRET")
	tick := func(prefix string, interval float64) tickworker.Options {
		return tickworker.Options{
			APIURL: apiURL, Secret: secret,
			Interval: milliseconds(settings.EnvPositive(prefix+"_INTERVAL_MS", interval)),
			Timeout:  milliseconds(settings.EnvPositive(prefix+"_TIMEOUT_MS", 60_000)),
		}
	}
	type runnable interface {
		Start() (supervisor.StopFunc, error)
		RunNow() bool
	}
	worker := func(name string, create func() (runnable, error)) supervisor.Service {
		return supervisor.Service{Name: name, Start: func(context.Context) (supervisor.StopFunc, error) {
			created, err := create()
			if err != nil {
				return nil, err
			}
			stop, err := created.Start()
			if err != nil {
				return nil, err
			}
			go created.RunNow()
			return stop, nil
		}}
	}
	if env("OUTBOUND_QUEUE_WORKER") != "off" {
		services = append(services, worker("outbound-queue", func() (runnable, error) {
			created, err := tickworker.NewOutboundQueue(tick("OUTBOUND_QUEUE", 30_000))
			if err == nil {
				session.mu.Lock()
				session.outbound = created
				session.mu.Unlock()
			}
			return created, err
		}))
	}
	if env("SNOOZE_WORKER") != "off" {
		services = append(services, worker("snooze-due", func() (runnable, error) {
			created, err := tickworker.NewSnooze(tick("SNOOZE", 30_000))
			if err == nil {
				session.mu.Lock()
				session.snooze = created
				session.mu.Unlock()
			}
			return created, err
		}))
	}
	if env("REMINDER_WORKER") != "off" {
		services = append(services, worker("follow-up-reminders", func() (runnable, error) {
			created, err := tickworker.NewFollowUpReminders(tick("REMINDER", 30_000))
			if err == nil {
				session.mu.Lock()
				session.reminders = created
				session.mu.Unlock()
			}
			return created, err
		}))
	}
	if env("PROVIDER_NOTIFICATION_WORKER") != "off" {
		services = append(services, worker("provider-new-mail", func() (runnable, error) {
			created, err := tickworker.NewProviderNotification(tick("PROVIDER_NOTIFICATION", 15_000))
			if err == nil {
				session.mu.Lock()
				session.provider = created
				session.mu.Unlock()
			}
			return created, err
		}))
	}
	return services
}

// reportListenerFailure turns a listener failure after startup into a
// supervised service failure, as `server.once('error', ...)` did.
func reportListenerFailure(session *Session, name string, errs <-chan error) {
	for err := range errs {
		if current := session.Supervisor(); current != nil && err != nil {
			_ = current.ReportFailure(name, err)
		}
		return
	}
}
