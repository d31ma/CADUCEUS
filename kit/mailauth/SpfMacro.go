package mailauth

import (
	"encoding/hex"
	"net"
	"regexp"
	"strings"
)

type spfMacro struct {
	values map[string]string
}

func newSpfMacro(identity spfIdentity, remote net.IP, domain string) spfMacro {
	macro := spfMacro{}
	macro.values = map[string]string{
		"%%": "%", "%_": " ", "%-": "%20",
		"%{d}": domain, "%{s}": identity.sender, "%{l}": identity.local,
		"%{o}": identity.domain, "%{i}": macro.address(remote), "%{h}": identity.helo,
	}
	return macro
}

func (macro spfMacro) expand(spec string) (string, error) {
	err := macro.supported(spec)
	if err != nil {
		return "", err
	}
	expanded := macro.token().ReplaceAllStringFunc(spec, macro.value)
	return macro.domain(expanded)
}

func (spfMacro) supported(spec string) error {
	if !regexp.MustCompile(`^(?:[^%]|%%|%_|%-|%\{[dslohi]\})*$`).MatchString(spec) {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (spfMacro) token() *regexp.Regexp {
	return regexp.MustCompile(`%%|%_|%-|%\{[dslohi]\}`)
}

func (macro spfMacro) value(token string) string {
	return macro.values[token]
}

func (spfMacro) domain(expanded string) (string, error) {
	domain := strings.ToLower(strings.TrimSuffix(expanded, "."))
	if domain == "" {
		return "", failure{result: Outcomes{}.PermError()}
	}
	return domain, nil
}

func (spfMacro) address(remote net.IP) string {
	if remote.To4() != nil {
		return remote.To4().String()
	}
	return strings.Join(strings.Split(hex.EncodeToString(remote.To16()), ""), ".")
}
