package smtpd

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// Arg is one MAIL FROM or RCPT TO parameter. Flag is true for a bare keyword
// such as SMTPUTF8; otherwise Value holds the xtext-decoded value.
type Arg struct {
	Value string
	Flag  bool
}

// Address is a parsed MAIL FROM or RCPT TO path. Address is empty for the
// null reverse path `<>`.
type Address struct {
	Address string
	Args    map[string]Arg
}

var (
	pathPattern       = regexp.MustCompile(`^<[^<>]*>$`)
	xtextPattern      = regexp.MustCompile(`\+([0-9A-F]{2})`)
	whitespacePattern = regexp.MustCompile(`\s+`)
	domainPattern     = regexp.MustCompile(`^[a-zA-Z0-9\x{0080}-\x{FFFF}.-]+$`)
)

func hasControl(value string) bool {
	for _, character := range value {
		if character <= 0x1f || (character >= 0x7f && character <= 0x9f) {
			return true
		}
	}
	return false
}

func hasUnsafeAddressCharacter(value string) bool {
	for _, character := range value {
		if character <= 0x1f || (character >= 0x7f && character <= 0x9f) ||
			character == 0x200b || character == 0x2060 || character == 0xfeff {
			return true
		}
	}
	return false
}

func decodeXtext(value string) string {
	return xtextPattern.ReplaceAllStringFunc(value, func(match string) string {
		code, _ := strconv.ParseUint(match[1:], 16, 8)
		return string(rune(code))
	})
}

// parseAddressCommand ports smtp-server's _parseAddressCommand with strict
// (non-lenient) address validation. It answers ok=false where the JavaScript
// answered false.
func parseAddressCommand(name, command string) (Address, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))
	parts := strings.Split(command, ":")
	head := strings.ToUpper(strings.TrimSpace(parts[0]))
	rest := whitespacePattern.Split(strings.TrimSpace(strings.Join(parts[1:], ":")), -1)
	if head != name {
		return Address{}, false
	}
	address := rest[0]
	invalid := false
	if !pathPattern.MatchString(address) {
		invalid = true
	} else {
		address = address[1 : len(address)-1]
	}
	var args map[string]Arg
	for _, part := range rest[1:] {
		pieces := strings.Split(part, "=")
		key := strings.ToUpper(pieces[0])
		if strings.TrimSpace(key) == "" {
			continue
		}
		value := strings.Join(pieces[1:], "=")
		arg := Arg{Flag: value == ""}
		if !arg.Flag {
			arg.Value = decodeXtext(value)
			if hasControl(arg.Value) {
				invalid = true
				continue
			}
		}
		if args == nil {
			args = map[string]Arg{}
		}
		args[key] = arg
	}
	if address != "" && hasUnsafeAddressCharacter(address) {
		invalid = true
	} else if address != "" {
		normalized, ok := validateMailbox(address)
		if ok {
			address = normalized
		} else {
			invalid = true
		}
	}
	if invalid {
		return Address{}, false
	}
	return Address{Address: address, Args: args}, true
}

func validateMailbox(address string) (string, bool) {
	pieces := strings.Split(address, "@")
	if len(pieces) != 2 || pieces[0] == "" || pieces[1] == "" {
		return "", false
	}
	local, domain := pieces[0], pieces[1]
	// RFC 5321 4.5.3.1.3: the path holds at most 256 octets with < @ >.
	if len(local)+1+len(domain) > 253 {
		return "", false
	}
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, "..") {
		return "", false
	}
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		content := domain[1 : len(domain)-1]
		if strings.HasPrefix(strings.ToUpper(content), "IPV6:") {
			parsed, err := netip.ParseAddr(content[5:])
			if err != nil || !parsed.Is6() || parsed.Zone() != "" {
				return "", false
			}
			return local + "@[IPv6:" + parsed.String() + "]", true
		}
		parsed, err := netip.ParseAddr(content)
		if err != nil || !parsed.Is4() {
			return "", false
		}
		return local + "@" + domain, true
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") ||
		strings.Contains(domain, ".-") || strings.Contains(domain, "-.") || !domainPattern.MatchString(domain) {
		return "", false
	}
	unicode, ok := domainToUnicode(domain)
	if !ok {
		return "", false
	}
	return local + "@" + unicode, true
}

// domainToUnicode decodes the `xn--` labels of a domain, as punycode.js's
// toUnicode does. Other labels are kept unchanged.
func domainToUnicode(domain string) (string, bool) {
	labels := strings.Split(domain, ".")
	for index, label := range labels {
		if len(label) > 4 && strings.EqualFold(label[:4], "xn--") {
			decoded, ok := punycodeDecode(strings.ToLower(label[4:]))
			if !ok {
				return "", false
			}
			labels[index] = decoded
		}
	}
	return strings.Join(labels, "."), true
}

// punycodeDecode implements the RFC 3492 decoder.
func punycodeDecode(input string) (string, bool) {
	const (
		base        = 36
		tMin        = 1
		tMax        = 26
		skew        = 38
		damp        = 700
		initialBias = 72
		initialN    = 128
	)
	adapt := func(delta, points int, first bool) int {
		if first {
			delta /= damp
		} else {
			delta /= 2
		}
		delta += delta / points
		k := 0
		for delta > ((base-tMin)*tMax)/2 {
			delta /= base - tMin
			k += base
		}
		return k + (base-tMin+1)*delta/(delta+skew)
	}
	var output []rune
	basic := strings.LastIndex(input, "-")
	if basic > 0 {
		for _, character := range input[:basic] {
			if character >= 0x80 {
				return "", false
			}
			output = append(output, character)
		}
		input = input[basic+1:]
	} else if basic == 0 {
		input = input[1:]
	}
	n, i, bias := initialN, 0, initialBias
	for position := 0; position < len(input); {
		oldI, w := i, 1
		for k := base; ; k += base {
			if position >= len(input) {
				return "", false
			}
			character := input[position]
			position++
			var digit int
			switch {
			case character >= '0' && character <= '9':
				digit = int(character-'0') + 26
			case character >= 'a' && character <= 'z':
				digit = int(character - 'a')
			case character >= 'A' && character <= 'Z':
				digit = int(character - 'A')
			default:
				return "", false
			}
			i += digit * w
			t := k - bias
			if t < tMin {
				t = tMin
			} else if t > tMax {
				t = tMax
			}
			if digit < t {
				break
			}
			w *= base - t
			if w > 1<<24 || i > 1<<30 {
				return "", false
			}
		}
		bias = adapt(i-oldI, len(output)+1, oldI == 0)
		n += i / (len(output) + 1)
		i %= len(output) + 1
		if n > 0x10ffff {
			return "", false
		}
		output = append(output[:i], append([]rune{rune(n)}, output[i:]...)...)
		i++
	}
	return string(output), true
}
