import "strings"

// NameUtils derives the labels the app shows for a person.
type NameUtils struct{}

// DisplayName prefers the stored name; otherwise it title-cases the local part
// of the address: jane.doe@example.com becomes "Jane Doe".
func (NameUtils) DisplayName(name string, email string) string {
	return NameUtils{}.preferred(strings.TrimSpace(name), NameUtils{}.humanized(email))
}

func (NameUtils) preferred(name string, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

func (NameUtils) humanized(email string) string {
	local := strings.SplitN(email, "@", 2)[0]
	parts := strings.FieldsFunc(local, NameUtils{}.separates)
	return strings.Join(NameUtils{}.titled(parts), " ")
}

func (NameUtils) separates(character rune) bool {
	return strings.ContainsRune("._- \t\n", character)
}

func (NameUtils) titled(parts []string) []string {
	titled := []string{}
	for _, part := range parts {
		titled = append(titled, strings.ToUpper(part[:1])+strings.ToLower(part[1:]))
	}
	return titled
}
