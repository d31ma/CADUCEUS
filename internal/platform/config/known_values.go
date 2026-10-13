package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
)

func (cfg Config) validateKnownValues() error {
	for _, validation := range []func() error{
		cfg.validateBooleans,
		cfg.validatePorts,
		cfg.validateIntegers,
		cfg.validateEnums,
		cfg.validateFormats,
		cfg.validateProductionSafety,
	} {
		if err := validation(); err != nil {
			return err
		}
	}
	return nil
}

func (cfg Config) validateBooleans() error {
	for key, value := range map[string]string{
		"FYLO_POSIX_ACCESS":           cfg.FYLOPosixAccess,
		"MAIL_SERVER_ENABLED":         cfg.MailServerEnabled,
		"MAIL_SUBMISSION_ENABLED":     cfg.MailSubmissionEnabled,
		"MAIL_ACME_ENABLED":           cfg.MailACMEEnabled,
		"MAIL_ACME_STAGING":           cfg.MailACMEStaging,
		"SPAM_FILTER_DISABLED":        cfg.SpamFilterDisabled,
		"WEB_PUSH_DISABLED":           cfg.WebPushDisabled,
		"CADUCEUS_ENABLE_TEST_ROUTES": cfg.EnableTestRoutes,
	} {
		if err := validateOptionalEnum(key, value, "true", "false"); err != nil {
			return err
		}
	}
	return nil
}

func (cfg Config) validatePorts() error {
	for key, value := range map[string]string{
		"SMTP_INBOUND_PORT":        cfg.SMTPInboundPort,
		"SMTP_SUBMISSION_PORT":     cfg.SMTPSubmissionPort,
		"SMTP_SUBMISSION_TLS_PORT": cfg.SMTPSubmissionTLSPort,
		"MAIL_ACME_HTTP_PORT":      cfg.MailACMEHTTPPort,
	} {
		if value == "" {
			continue
		}
		if _, err := ParsePort(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (cfg Config) validateIntegers() error {
	for key, value := range map[string]string{
		"SMTP_MAX_SIZE":               cfg.SMTPMaxSize,
		"OUTBOUND_QUEUE_INTERVAL_MS":  cfg.OutboundQueueIntervalMS,
		"OUTBOUND_QUEUE_TIMEOUT_MS":   cfg.OutboundQueueTimeoutMS,
		"SNOOZE_INTERVAL_MS":          cfg.SnoozeIntervalMS,
		"SNOOZE_TIMEOUT_MS":           cfg.SnoozeTimeoutMS,
		"REMINDER_INTERVAL_MS":        cfg.ReminderIntervalMS,
		"REMINDER_TIMEOUT_MS":         cfg.ReminderTimeoutMS,
		"STREAM_MAX_MS":               cfg.StreamMaxMS,
		"STREAM_POLL_MS":              cfg.StreamPollMS,
		"STREAM_REPLAY_LIMIT":         cfg.StreamReplayLimit,
		"FYLO_MAX_REQUEST_BYTES":      cfg.FYLOMaxRequestBytes,
		"FYLO_MAX_RESPONSE_BYTES":     cfg.FYLOMaxResponseBytes,
		"FYLO_RESTART_LIMIT":          cfg.FYLORestartLimit,
		"FYLO_MAX_ATTACHMENT_BYTES":   cfg.FYLOMaxAttachmentBytes,
		"FYLO_ATTACHMENT_QUOTA_BYTES": cfg.FYLOAttachmentQuota,
	} {
		if err := validateOptionalInteger(key, value, false); err != nil {
			return err
		}
	}
	if err := validateOptionalInteger("MAIL_STATUS_INTERVAL_MS", cfg.MailStatusIntervalMS, true); err != nil {
		return err
	}
	return nil
}

func (cfg Config) validateEnums() error {
	for key, validation := range map[string]struct {
		value   string
		allowed []string
	}{
		"CADUCEUS_AUTH_POLICY":  {cfg.AuthPolicy, []string{"passkey-first", "password-and-passkey"}},
		"MAIL_AUTH_VERIFY":      {cfg.MailAuthVerify, []string{"on", "off"}},
		"SMTP_ADAPTER":          {cfg.SMTPAdapter, []string{"direct", "console"}},
		"SMTP_TLS_POLICY":       {cfg.SMTPTLSPolicy, []string{"opportunistic", "require"}},
		"OUTBOUND_QUEUE_WORKER": {cfg.OutboundQueueWorker, []string{"on", "off"}},
		"SNOOZE_WORKER":         {cfg.SnoozeWorker, []string{"on", "off"}},
		"REMINDER_WORKER":       {cfg.ReminderWorker, []string{"on", "off"}},
		"NODE_ENV":              {cfg.NodeEnv, []string{"development", "test", "production"}},
	} {
		if err := validateOptionalEnum(key, validation.value, validation.allowed...); err != nil {
			return err
		}
	}
	return nil
}

func (cfg Config) validateFormats() error {
	if err := validateOptionalURL("PUBLIC_URL", cfg.PublicURL); err != nil {
		return err
	}
	for key, value := range map[string]string{
		"SMTP_FROM":       cfg.SMTPFrom,
		"MAIL_ACME_EMAIL": cfg.MailACMEEmail,
	} {
		if err := validateOptionalEmail(key, value); err != nil {
			return err
		}
	}
	if cfg.SpamScoreThreshold != "" {
		score, err := strconv.ParseFloat(cfg.SpamScoreThreshold, 64)
		if err != nil || score < 0 {
			return fmt.Errorf("SPAM_SCORE_THRESHOLD must be a non-negative decimal")
		}
	}
	return nil
}

func (cfg Config) validateProductionSafety() error {
	if cfg.NodeEnv != "production" {
		return nil
	}
	if cfg.SMTPAdapter == "console" {
		return fmt.Errorf("SMTP_ADAPTER=console is forbidden in production")
	}
	if cfg.EnableTestRoutes == "true" {
		return fmt.Errorf("CADUCEUS_ENABLE_TEST_ROUTES=true is forbidden in production")
	}
	if cfg.MailACMEStaging == "true" {
		return fmt.Errorf("MAIL_ACME_STAGING=true is forbidden in production")
	}
	return nil
}

func validateOptionalEnum(key, value string, allowed ...string) error {
	if value == "" {
		return nil
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of %s", key, strings.Join(allowed, ", "))
}

func validateOptionalInteger(key, value string, allowZero bool) error {
	if value == "" {
		return nil
	}
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil || (!allowZero && number == 0) {
		kind := "a positive integer"
		if allowZero {
			kind = "a non-negative integer"
		}
		return fmt.Errorf("%s must be %s", key, kind)
	}
	return nil
}

func validateOptionalURL(key, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", key)
	}
	return nil
}

func validateOptionalEmail(key, value string) error {
	if value == "" {
		return nil
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return fmt.Errorf("%s must be an email address", key)
	}
	return nil
}
