import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitHubProfiles is the optional, privacy-gated GitHub contact enrichment.
// The caller decides whether the mailbox opted in; this type talks only to
// the fixed public GitHub API origin, follows no redirect, accepts a small
// allowlisted response shape and derives every URL it answers itself.
// Lookups are remembered for an hour (ten minutes when nothing was found).
type GitHubProfiles struct {
	Cache *GitHubProfileCache
}

// FindByEmail answers {status: found, profile} for the first public GitHub
// user with the exact address, {status: not-found}, or {status: unavailable}
// when GitHub could not be asked.
func (profiles GitHubProfiles) FindByEmail(email string) map[string]any {
	address := Security{}.NormalizeEmailAddress(email)
	if address == "" {
		return map[string]any{"status": "not-found"}
	}
	if cached, found := profiles.Cache.Lookup(address); found {
		return cached
	}
	users, err := profiles.request("/search/users?q=" + profiles.encode(address) + "&per_page=1")
	if err != nil {
		return map[string]any{"status": "unavailable"}
	}
	items, _ := users["items"].([]any)
	first := map[string]any{}
	if len(items) > 0 {
		first, _ = items[0].(map[string]any)
	}
	login := profiles.cleanLogin(first["login"])
	if login == "" {
		return profiles.Cache.Remember(address, map[string]any{"status": "not-found"}, 10*time.Minute)
	}
	found, err := profiles.request("/search/repositories?q=" + profiles.encode("user:"+login) + "&sort=stars&order=desc&per_page=5")
	if err != nil {
		return map[string]any{"status": "unavailable"}
	}
	profile := map[string]any{"login": login, "profileUrl": "https://github.com/" + login, "repositories": profiles.repositories(found["items"], login)}
	return profiles.Cache.Remember(address, map[string]any{"status": "found", "profile": profile}, time.Hour)
}

func (profiles GitHubProfiles) repositories(value any, login string) []map[string]any {
	items, _ := value.([]any)
	repositories := []map[string]any{}
	for _, item := range items {
		repository, _ := item.(map[string]any)
		if sanitized := profiles.repository(repository, login); sanitized != nil && len(repositories) < 5 {
			repositories = append(repositories, sanitized)
		}
	}
	sort.SliceStable(repositories, func(left, right int) bool {
		a, b := repositories[left], repositories[right]
		if a["stars"] != b["stars"] {
			return a["stars"].(float64) > b["stars"].(float64)
		}
		return TextOrder{}.Compare(a["fullName"].(string), b["fullName"].(string)) < 0
	})
	return repositories
}

func (profiles GitHubProfiles) repository(repository map[string]any, login string) map[string]any {
	name := profiles.cleanText(repository["name"], 100)
	ownerObject, _ := repository["owner"].(map[string]any)
	owner := profiles.cleanLogin(ownerObject["login"])
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`).MatchString(name) || owner == "" || !strings.EqualFold(owner, login) {
		return nil
	}
	stars := 0.0
	if count, isNumber := repository["stargazers_count"].(float64); isNumber && count == math.Trunc(count) && count >= 0 && count <= 9007199254740991 {
		stars = math.Min(count, 2_147_483_647)
	}
	return map[string]any{
		"name":        name,
		"fullName":    owner + "/" + name,
		"url":         "https://github.com/" + owner + "/" + name,
		"description": profiles.cleanText(repository["description"], 300),
		"language":    profiles.cleanText(repository["language"], 50),
		"stars":       stars,
	}
}

// request answers one bounded GitHub API object.
func (GitHubProfiles) request(path string) (map[string]any, error) {
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects are refused") },
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com"+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "CADUCEUS-Mail-Server")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("GitHub answered %d", response.StatusCode)
	}
	maximum := int64(1_000_000)
	if declared, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64); declared > maximum {
		return nil, errors.New("GitHub response is too large")
	}
	text, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil || int64(len(text)) > maximum {
		return nil, errors.New("GitHub response is too large")
	}
	var parsed map[string]any
	if json.Unmarshal(text, &parsed) != nil || parsed == nil {
		return nil, errors.New("Invalid GitHub response")
	}
	return parsed, nil
}

func (GitHubProfiles) cleanLogin(value any) string {
	text, _ := value.(string)
	login := Security{}.Trim(text)
	if !regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`).MatchString(login) {
		return ""
	}
	return login
}

func (GitHubProfiles) cleanText(value any, maximum int) string {
	text, isText := value.(string)
	if !isText {
		return ""
	}
	text = regexp.MustCompile(`[\x00-\x1f\x7f]`).ReplaceAllString(text, " ")
	return TextValue{}.Prefix(Security{}.Trim(TextValue{}.Collapsed(text)), maximum)
}

// encode answers encodeURIComponent(value).
func (GitHubProfiles) encode(value string) string {
	var encoded strings.Builder
	for _, character := range []byte(value) {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || strings.IndexByte("-_.!~*'()", character) >= 0 {
			encoded.WriteByte(character)
		} else {
			fmt.Fprintf(&encoded, "%%%02X", character)
		}
	}
	return encoded.String()
}
