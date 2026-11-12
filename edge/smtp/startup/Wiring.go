package startup

import (
	"crypto/tls"
	"log"
	"net/http"
	"time"

	"mesail.dev/edge/smtp/adapters"
	"mesail.dev/edge/smtp/mailchecks"
	"mesail.dev/edge/smtp/protocol"
	"mesail.dev/edge/smtp/reception"
	"mesail.dev/packages/mailauth"
	"mesail.dev/packages/mime"
)

type Wiring struct{}

func (wiring Wiring) Build(settings ListenerSettings, logger *log.Logger) (protocol.Settings, error) {
	tlsSettings, err := wiring.tlsFor(settings)(settings)
	if err != nil {
		return protocol.Settings{}, err
	}
	client := &http.Client{Timeout: wiring.requestTimeout()}
	tokens := wiring.tokensFor(settings, client)
	store, err := RawStoreFactory{}.Store(settings)
	if err != nil {
		return protocol.Settings{}, err
	}
	delivery := reception.NewReception(reception.Parts{
		Checker: wiring.checkerFor(settings),
		Headers: mailchecks.NewMimeHeaderReader(mime.Parser{}),
		Store:   store,
		MailIn:  adapters.NewHttpMailIn(settings.MailInUrl, tokens, client),
	})
	return protocol.Settings{
		Hostname:       settings.Hostname,
		MaxSize:        settings.MaxSize,
		MaxConnections: settings.MaxConnections,
		Tls:            tlsSettings,
		Directory:      adapters.NewHttpDirectory(settings.DirectoryUrl, tokens, client),
		Delivery:       delivery,
		Log:            logger,
	}, nil
}

func (wiring Wiring) tlsFor(settings ListenerSettings) func(ListenerSettings) (*tls.Config, error) {
	if settings.CertificateFile == "" {
		return wiring.noTls
	}
	return wiring.certificateTls
}

func (Wiring) noTls(_ ListenerSettings) (*tls.Config, error) {
	return nil, nil
}

func (Wiring) certificateTls(settings ListenerSettings) (*tls.Config, error) {
	files := protocol.NewCertificateFiles(settings.CertificateFile, settings.KeyFile)
	_, err := files.Certificate(nil)
	return files.TlsSettings(), err
}

func (Wiring) tokensFor(settings ListenerSettings, client *http.Client) reception.ServiceTokens {
	if settings.ServiceToken != "" {
		return adapters.NewStaticTokens(settings.ServiceToken)
	}
	return adapters.NewIdentityTokens(settings.TokenSettings, client, time.Now)
}

func (Wiring) checkerFor(settings ListenerSettings) reception.AuthChecker {
	if !settings.VerifyAuth {
		return reception.SkippedChecks{}
	}
	return mailchecks.NewMailAuthChecker(mailauth.NewChecker(mailauth.NewNetResolver()), settings.Hostname)
}

func (Wiring) requestTimeout() time.Duration {
	return 30 * time.Second
}
