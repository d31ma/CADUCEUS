import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"sort"
	"strings"
)

// FyloAccess is the POSIX identity a FYLO operation runs as when the
// deployment enables FYLO_POSIX_ACCESS: the mailbox user and the groups of
// its domains. Without that setting operations carry no access descriptor.
type FyloAccess struct {
	Uid    int   `json:"uid"`
	Groups []int `json:"groups"`
}

// ForMailbox answers the access of one mailbox, or nil when POSIX access is
// off or the claims name no mailbox and domain.
func (FyloAccess) ForMailbox(email string, domains []string) *FyloAccess {
	access := FyloAccess{}
	if !access.enabled() || strings.TrimSpace(email) == "" || len(domains) == 0 {
		return nil
	}
	return &FyloAccess{Uid: access.posixId("mailbox-user", strings.ToLower(strings.TrimSpace(email))), Groups: access.groups(domains)}
}

// ForDomain answers the service access to one domain's tenant storage.
func (FyloAccess) ForDomain(domain string) *FyloAccess {
	access := FyloAccess{}
	if !access.enabled() {
		return nil
	}
	return &FyloAccess{Uid: os.Getuid(), Groups: access.groups([]string{domain})}
}

func (FyloAccess) enabled() bool {
	return Settings{}.Value("FYLO_POSIX_ACCESS") == "true"
}

func (access FyloAccess) groups(domains []string) []int {
	seen := map[string]bool{}
	var groups []int
	for _, domain := range domains {
		normalized := strings.ToLower(strings.TrimSpace(domain))
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			groups = append(groups, access.posixId("mail-domain", normalized))
		}
	}
	sort.Ints(groups)
	return groups
}

// posixId is the stable identifier FYLO uses for one name in a namespace.
func (access FyloAccess) posixId(namespace string, key string) int {
	digest := sha256.Sum256([]byte(namespace + "\x00" + key))
	return access.minimumId() + int(binary.BigEndian.Uint32(digest[:4])%uint32(access.idSpan()))
}

func (FyloAccess) minimumId() int { return 100_000 }

func (FyloAccess) idSpan() int { return 2_000_000_000 }
