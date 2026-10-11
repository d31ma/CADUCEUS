import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// RemoteImages finds the network images a stored message references and
// fetches them for the reader without the browser's identity: no cookies,
// no referrer, public addresses only (re-resolved on every redirect and
// pinned for the connection), standard ports, raster image types with a
// matching signature, and at most 4 MiB each.
type RemoteImages struct{}

// MostPerRequest is how many images one request may fetch.
func (RemoteImages) MostPerRequest() int { return 6 }

func (RemoteImages) largestImage() int64 { return 4 * 1024 * 1024 }

func (RemoteImages) mostRedirects() int { return 3 }

func (RemoteImages) timeout() time.Duration { return 10 * time.Second }

// Urls answers the hrefs of the images the HTML references (src,
// background, poster, srcset and CSS url() values), at most 64.
func (images RemoteImages) Urls(value any) []string {
	document, _ := value.(string)
	if document == "" {
		return []string{}
	}
	parsed := HtmlAttributes{}.Parsed(document)
	found := []string{}
	add := func(candidate string) {
		if href := (WebAddress{}).Href(candidate); href != "" && !(TextValue{}).Includes(found, href) && len(found) < 64 {
			found = append(found, href)
		}
	}
	styleUrl := regexp.MustCompile(`(?i)url\(\s*['"]?([^'")]+)['"]?\s*\)`)
	for _, attributes := range parsed.Tags {
		for _, name := range []string{"src", "background", "poster"} {
			add(attributes[name])
		}
		for _, entry := range strings.Split(attributes["srcset"], ",") {
			if fields := strings.Fields(TextValue{}.Trim(entry)); len(fields) > 0 {
				add(fields[0])
			}
		}
		for _, match := range styleUrl.FindAllStringSubmatch(attributes["style"], -1) {
			add(match[1])
		}
	}
	for _, style := range parsed.Styles {
		for _, match := range styleUrl.FindAllStringSubmatch(style, -1) {
			add(match[1])
		}
	}
	return found
}

// Fetched answers one image as {url, contentType, contentBase64, size}.
func (images RemoteImages) Fetched(source string) (map[string]any, error) {
	current, err := images.target(source)
	for redirect := 0; err == nil && redirect <= images.mostRedirects(); redirect++ {
		var response *http.Response
		if response, err = images.requested(current); err != nil {
			break
		}
		location := response.Header.Get("Location")
		switch response.StatusCode {
		case 301, 302, 303, 307, 308:
			response.Body.Close()
			if location == "" || redirect == images.mostRedirects() {
				return nil, errors.New("Remote image redirect rejected")
			}
			next, parseErr := current.Parse(location)
			if parseErr != nil {
				return nil, errors.New("Remote image redirect rejected")
			}
			current, err = images.target(next.String())
			continue
		}
		return images.image(source, response)
	}
	if err == nil {
		err = errors.New("Remote image redirect rejected")
	}
	return nil, err
}

func (images RemoteImages) target(value string) (*url.URL, error) {
	href := WebAddress{}.Href(value)
	if href == "" {
		return nil, errors.New("Remote image URL must use HTTP or HTTPS")
	}
	parsed, _ := url.Parse(href)
	if parsed.User != nil {
		return nil, errors.New("Remote image URL must not contain credentials")
	}
	if port := parsed.Port(); port != "" && port != "80" && port != "443" {
		return nil, errors.New("Remote image URL must use a standard web port")
	}
	return parsed, nil
}

// requested resolves the host, refuses any non-public address, and sends the
// request to the first address.
func (images RemoteImages) requested(target *url.URL) (*http.Response, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(context.Background(), target.Hostname())
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("Remote image host did not resolve to public addresses")
	}
	for _, address := range addresses {
		if !images.IsPublic(address.IP) {
			return nil, errors.New("Remote image host did not resolve to public addresses")
		}
	}
	port := target.Port()
	if port == "" {
		port = map[string]string{"https": "443", "http": "80"}[target.Scheme]
	}
	pinned := net.JoinHostPort(addresses[0].IP.String(), port)
	client := &http.Client{
		Timeout: images.timeout(),
		Transport: &http.Transport{
			Proxy:              nil,
			DisableCompression: true,
			DialContext: func(ctx context.Context, network string, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: images.timeout()}).DialContext(ctx, network, pinned)
			},
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	request, err := http.NewRequest(http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/gif")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "CADUCEUS-Remote-Image-Proxy/1.0")
	return client.Do(request)
}

func (images RemoteImages) image(source string, response *http.Response) (map[string]any, error) {
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("Remote image request failed")
	}
	if response.ContentLength > images.largestImage() {
		return nil, errors.New("Remote image exceeds the size limit")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, images.largestImage()+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > images.largestImage() {
		return nil, errors.New("Remote image exceeds the size limit")
	}
	contentType := strings.ToLower(TextValue{}.Trim(strings.SplitN(response.Header.Get("Content-Type"), ";", 2)[0]))
	if !images.signed(body, contentType) {
		return nil, errors.New("Remote resource is not a supported raster image")
	}
	return map[string]any{
		"url": WebAddress{}.Href(source), "contentType": contentType,
		"contentBase64": base64.StdEncoding.EncodeToString(body), "size": len(body),
	}, nil
}

// signed reports whether bytes carry the signature of a supported type.
func (RemoteImages) signed(content []byte, contentType string) bool {
	switch contentType {
	case "image/png":
		return bytes.HasPrefix(content, []byte{137, 80, 78, 71, 13, 10, 26, 10})
	case "image/jpeg":
		return len(content) >= 2 && content[0] == 0xff && content[1] == 0xd8 && bytes.HasSuffix(content, []byte{0xff, 0xd9})
	case "image/gif":
		return bytes.HasPrefix(content, []byte("GIF87a")) || bytes.HasPrefix(content, []byte("GIF89a"))
	case "image/webp":
		return len(content) >= 12 && string(content[0:4]) == "RIFF" && string(content[8:12]) == "WEBP"
	case "image/avif":
		return len(content) >= 12 && string(content[4:8]) == "ftyp" && (string(content[8:12]) == "avif" || string(content[8:12]) == "avis")
	}
	return false
}

// IsPublic refuses local, private, link-local, multicast, documentation,
// benchmarking and other non-global addresses of both families.
func (RemoteImages) IsPublic(address net.IP) bool {
	if v4 := address.To4(); v4 != nil {
		a, b, c := v4[0], v4[1], v4[2]
		switch {
		case a == 0 || a == 10 || a == 127 || a >= 224:
			return false
		case a == 100 && b >= 64 && b <= 127, a == 169 && b == 254, a == 172 && b >= 16 && b <= 31:
			return false
		case a == 192 && b == 0 && (c == 0 || c == 2), a == 192 && b == 168:
			return false
		case a == 198 && (b == 18 || b == 19), a == 198 && b == 51 && c == 100, a == 203 && b == 0 && c == 113:
			return false
		}
		return true
	}
	_, global, _ := net.ParseCIDR("2000::/3")
	_, documentation, _ := net.ParseCIDR("2001:db8::/32")
	return address.To16() != nil && global.Contains(address) && !documentation.Contains(address)
}
