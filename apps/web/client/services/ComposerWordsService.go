import (
	"slices"
	"strconv"
	"strings"
)

// @Service
type ComposerWordsService struct{}

func (words *ComposerWordsService) ChipLabel(recipient Recipient) string {
	return words.labels()[words.labelKey(recipient)]
}

func (words *ComposerWordsService) ChipAriaLabel(recipient Recipient) string {
	return strings.Join([]string{recipient.DisplayName, words.ChipLabel(recipient), recipient.Address}, words.comma()) + ". Open menu"
}

func (words *ComposerWordsService) ChatReason(kind string, address string, domainId string) string {
	reasons := map[string]string{
		words.kindOtherDomain(): "Chat stays inside " + domainId,
		words.kindExternal():    address + " has no Mesail account",
	}
	return reasons[kind]
}

func (words *ComposerWordsService) BothReason(kind string) string {
	reasons := map[string]string{
		words.kindOtherDomain(): "Not possible: this person is in another domain",
		words.kindExternal():    "Not possible without a Mesail account",
	}
	return reasons[kind]
}

func (words *ComposerWordsService) Summary(recipients []Recipient) []string {
	chatNames := words.namesOf(words.readyWith(recipients, words.deliveryChat()))
	emailNames := words.namesOf(words.readyWith(recipients, words.deliveryEmail()))
	bothNames := words.namesOf(words.readyWith(recipients, words.deliveryBoth()))
	lines := []string{
		words.sentence(words.joined(chatNames), words.verb(len(chatNames)), "a chat message"),
		words.sentence(words.joined(emailNames), words.verb(len(emailNames)), words.anEmail()),
		words.sentence(words.joined(bothNames), words.verb(len(bothNames)), "a chat message and an email"),
	}
	return slices.DeleteFunc(lines, words.isEmpty)
}

func (words *ComposerWordsService) SendLabel(recipients []Recipient) string {
	chats := words.countDeliveries(recipients, words.chatDeliveries())
	emails := words.countDeliveries(recipients, words.emailDeliveries())
	mixed := "Send · " + words.counted(chats, words.deliveryChat()) + words.comma() + words.counted(emails, words.deliveryEmail())
	labels := map[string]string{words.modeChat(): "Send", words.modeEmail(): "Send email", words.modeMixed(): mixed}
	return labels[words.sendMode(chats, emails)]
}

func (words *ComposerWordsService) EmailMode(recipients []Recipient) bool {
	return words.countDeliveries(recipients, words.emailDeliveries()) > 0
}

func (words *ComposerWordsService) ModeText(recipients []Recipient, domainId string) string {
	chats := words.countDeliveries(recipients, words.chatDeliveries())
	emails := words.countDeliveries(recipients, words.emailDeliveries())
	outside := words.joined(words.namesOf(words.outside(recipients)))
	emailNames := words.namesOf(words.readyEmail(recipients))
	texts := map[string]string{
		words.modeChat():  "",
		words.modeEmail(): "This is an email. " + words.outsideSentence(outside, words.isOrAre(outside), domainId),
		words.modeMixed(): "This is a chat and an email. Chat stays inside " + domainId + ", so " + words.sentence(words.joined(emailNames), words.verb(len(emailNames)), words.anEmail()),
	}
	return texts[words.sendMode(chats, emails)]
}

func (words *ComposerWordsService) FirstName(displayName string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(displayName), words.space())
	return first
}

func (words *ComposerWordsService) Initials(displayName string) string {
	return strings.ToUpper(words.initialsOf(strings.Fields(displayName)))
}

func (words *ComposerWordsService) labelKey(recipient Recipient) string {
	if recipient.State != words.stateReady() {
		return recipient.State
	}
	return recipient.Kind + words.colon() + recipient.Delivery
}

func (words *ComposerWordsService) chatLabel() string {
	return "Chat"
}

func (words *ComposerWordsService) emailLabel() string {
	return "Email"
}

func (words *ComposerWordsService) anEmail() string {
	return "an email"
}

func (words *ComposerWordsService) labels() map[string]string {
	return map[string]string{
		"checking":          "Checking…",
		"invalid":           "Not valid",
		"member:chat":       words.chatLabel(),
		"member:email":      words.emailLabel(),
		"member:both":       "Chat + email",
		"channel:chat":      words.chatLabel(),
		"otherDomain:email": "Email · other domain",
		"external:email":    words.emailLabel(),
	}
}

func (words *ComposerWordsService) space() string {
	return " "
}

func (words *ComposerWordsService) colon() string {
	return ":"
}

func (words *ComposerWordsService) joined(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], words.comma()) + words.and() + names[len(names)-1]
}

func (words *ComposerWordsService) comma() string {
	return ", "
}

func (words *ComposerWordsService) and() string {
	return " and "
}

func (words *ComposerWordsService) period() string {
	return "."
}

func (words *ComposerWordsService) sentence(names string, verb string, what string) string {
	if names == "" {
		return ""
	}
	return names + words.space() + verb + words.space() + what + words.period()
}

func (words *ComposerWordsService) verb(count int) string {
	return map[bool]string{true: "gets", false: "get"}[count == 1]
}

func (words *ComposerWordsService) isOrAre(names string) string {
	return map[bool]string{true: "are", false: "is"}[strings.Contains(names, words.and())]
}

func (words *ComposerWordsService) outsideSentence(outside string, verb string, domainId string) string {
	if outside == "" {
		return "Everyone gets it in their mailbox."
	}
	return outside + words.space() + verb + " outside " + domainId + words.period()
}

func (words *ComposerWordsService) readyWith(recipients []Recipient, delivery string) []Recipient {
	wanted := delivery + words.colon() + words.stateReady()
	return slices.DeleteFunc(slices.Clone(recipients), func(recipient Recipient) bool {
		return recipient.Delivery+words.colon()+recipient.State != wanted
	})
}

func (words *ComposerWordsService) readyEmail(recipients []Recipient) []Recipient {
	return slices.DeleteFunc(slices.Clone(recipients), func(recipient Recipient) bool {
		return !words.readyEmailKeys()[recipient.Delivery+words.colon()+recipient.State]
	})
}

func (words *ComposerWordsService) outside(recipients []Recipient) []Recipient {
	return slices.DeleteFunc(slices.Clone(recipients), func(recipient Recipient) bool {
		return !words.outsideKinds()[recipient.Kind]
	})
}

func (words *ComposerWordsService) namesOf(recipients []Recipient) []string {
	names := []string{}
	for _, recipient := range recipients {
		names = append(names, recipient.DisplayName)
	}
	return names
}

func (words *ComposerWordsService) isEmpty(line string) bool {
	return line == ""
}

func (words *ComposerWordsService) countDeliveries(recipients []Recipient, counts map[string]int) int {
	total := 0
	for _, recipient := range recipients {
		total += counts[recipient.Delivery]
	}
	return total
}

func (words *ComposerWordsService) modeChat() string {
	return "chatOnly"
}

func (words *ComposerWordsService) modeEmail() string {
	return "emailOnly"
}

func (words *ComposerWordsService) modeMixed() string {
	return "mixed"
}

func (words *ComposerWordsService) sendMode(chats int, emails int) string {
	switch {
	case emails == 0:
		return words.modeChat()
	case chats == 0:
		return words.modeEmail()
	}
	return words.modeMixed()
}

func (words *ComposerWordsService) counted(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(count) + words.space() + noun + "s"
}

func (words *ComposerWordsService) initialsOf(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return string([]rune(parts[0])[:1])
	}
	return string([]rune(parts[0])[:1]) + string([]rune(parts[len(parts)-1])[:1])
}

func (words *ComposerWordsService) deliveryChat() string {
	return "chat"
}

func (words *ComposerWordsService) deliveryEmail() string {
	return "email"
}

func (words *ComposerWordsService) deliveryBoth() string {
	return "both"
}

func (words *ComposerWordsService) chatDeliveries() map[string]int {
	return map[string]int{words.deliveryChat(): 1, words.deliveryBoth(): 1}
}

func (words *ComposerWordsService) emailDeliveries() map[string]int {
	return map[string]int{words.deliveryEmail(): 1, words.deliveryBoth(): 1}
}

func (words *ComposerWordsService) readyEmailKeys() map[string]bool {
	return map[string]bool{
		words.deliveryEmail() + words.colon() + words.stateReady(): true,
		words.deliveryBoth() + words.colon() + words.stateReady():  true,
	}
}

func (words *ComposerWordsService) kindOtherDomain() string {
	return "otherDomain"
}

func (words *ComposerWordsService) kindExternal() string {
	return "external"
}

func (words *ComposerWordsService) stateReady() string {
	return "ready"
}

func (words *ComposerWordsService) outsideKinds() map[string]bool {
	return map[string]bool{words.kindOtherDomain(): true, words.kindExternal(): true}
}
