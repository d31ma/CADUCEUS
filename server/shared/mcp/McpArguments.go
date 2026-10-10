import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// McpArguments checks tool arguments as the tools' zod schemas did: unknown
// arguments are dropped, defaults fill missing ones, and every problem is
// reported with zod's wording and path.
type McpArguments struct{}

// Parsed answers the arguments a tool receives, or the zod issues as
// "path: message" texts.
func (arguments McpArguments) Parsed(tool McpTool, input map[string]any) (map[string]any, []string) {
	parsed := map[string]any{}
	issues := []string{}
	for _, field := range tool.Fields {
		value, present := input[field.Name]
		if !present {
			if field.Default != nil {
				parsed[field.Name] = arguments.copied(field.Default)
				continue
			}
			if field.Optional {
				continue
			}
		}
		checked, problems := arguments.checked(field, value, present, field.Name)
		issues = append(issues, problems...)
		if len(problems) == 0 {
			parsed[field.Name] = checked
		}
	}
	return parsed, issues
}

func (arguments McpArguments) checked(field McpToolField, value any, present bool, path string) (any, []string) {
	expected := map[string]string{"string": "string", "integer": "number", "boolean": "boolean", "array": "array"}[field.Kind]
	received := arguments.received(value, present)
	if received != expected {
		return nil, []string{fmt.Sprintf("%s: Invalid input: expected %s, received %s", path, expected, received)}
	}
	switch field.Kind {
	case "string":
		return value, arguments.text(field, value.(string), path)
	case "integer":
		number := value.(float64)
		if number != math.Trunc(number) {
			return nil, []string{path + ": Invalid input: expected int, received number"}
		}
		return value, arguments.bounds(field, int(number), "number to be", "", path)
	case "array":
		return arguments.items(field, value.([]any), path)
	}
	return value, nil
}

func (arguments McpArguments) items(field McpToolField, list []any, path string) (any, []string) {
	issues := []string{}
	items := []any{}
	for index, item := range list {
		checked, problems := arguments.checked(*field.Items, item, true, fmt.Sprintf("%s.%d", path, index))
		issues = append(issues, problems...)
		items = append(items, checked)
	}
	issues = append(issues, arguments.bounds(field, len(list), "array to have", " items", path)...)
	return items, issues
}

func (arguments McpArguments) text(field McpToolField, value string, path string) []string {
	issues := arguments.bounds(field, TextValue{}.Length(value), "string to have", " characters", path)
	switch field.Format {
	case "email":
		if !arguments.IsEmail(value) {
			issues = append(issues, path+": Invalid email address")
		}
	case "date-time":
		if !regexp.MustCompile(arguments.DateTimePattern()).MatchString(value) {
			issues = append(issues, path+": Invalid ISO datetime")
		}
	}
	return issues
}

func (McpArguments) bounds(field McpToolField, size int, subject string, unit string, path string) []string {
	if field.Minimum != nil && size < *field.Minimum {
		return []string{fmt.Sprintf("%s: Too small: expected %s >=%d%s", path, subject, *field.Minimum, unit)}
	}
	if field.Maximum != nil && size > *field.Maximum {
		return []string{fmt.Sprintf("%s: Too big: expected %s <=%d%s", path, subject, *field.Maximum, unit)}
	}
	return nil
}

// Received names the type of a JSON value as zod reported it.
func (McpArguments) received(value any, present bool) string {
	if !present {
		return "undefined"
	}
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	}
	return "object"
}

// IsEmail reports whether value is an address zod's email check accepts.
func (arguments McpArguments) IsEmail(value string) bool {
	return !strings.HasPrefix(value, ".") && !strings.Contains(value, "..") &&
		regexp.MustCompile(`^[A-Za-z0-9_'+\-.]*[A-Za-z0-9_+-]@([A-Za-z0-9][A-Za-z0-9\-]*\.)+[A-Za-z]{2,}$`).MatchString(value)
}

// EmailPattern is the pattern the email schemas publish.
func (McpArguments) EmailPattern() string {
	return `^(?!\.)(?!.*\.\.)([A-Za-z0-9_'+\-\.]*)[A-Za-z0-9_+-]@([A-Za-z0-9][A-Za-z0-9\-]*\.)+[A-Za-z]{2,}$`
}

// DateTimePattern is zod's ISO date-time pattern (UTC, seconds optional).
func (McpArguments) DateTimePattern() string {
	return `^(?:(?:\d\d[2468][048]|\d\d[13579][26]|\d\d0[48]|[02468][048]00|[13579][26]00)-02-29|\d{4}-(?:(?:0[13578]|1[02])-(?:0[1-9]|[12]\d|3[01])|(?:0[469]|11)-(?:0[1-9]|[12]\d|30)|(?:02)-(?:0[1-9]|1\d|2[0-8])))T(?:(?:[01]\d|2[0-3]):[0-5]\d(?::[0-5]\d(?:\.\d+)?)?(?:Z))$`
}

// copied answers a fresh copy of a default, so no call shares a list.
func (McpArguments) copied(value any) any {
	switch typed := value.(type) {
	case []any:
		return append([]any{}, typed...)
	case int:
		return float64(typed)
	}
	return value
}
