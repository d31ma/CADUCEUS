import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// @Service
type ConversationReactionService struct{}

func (service *ConversationReactionService) Reactions(counts map[string]int, mine []string) []MessageReaction {
	reactions := slices.DeleteFunc(service.all(counts, mine), service.empty)
	slices.SortFunc(reactions, service.order)
	return reactions
}

func (service *ConversationReactionService) Choices(mine []string) []MessageReaction {
	return service.choicesFor(service.kinds(), mine)
}

func (service *ConversationReactionService) all(counts map[string]int, mine []string) []MessageReaction {
	reactions := []MessageReaction{}
	for name, count := range counts {
		reactions = append(reactions, service.reaction(service.kindOf(name), count, slices.Contains(mine, name)))
	}
	return reactions
}

func (service *ConversationReactionService) reaction(kind MessageReaction, count int, isMine bool) MessageReaction {
	return MessageReaction{
		Emoji: kind.Emoji,
		Glyph: kind.Glyph,
		Count: count,
		Mine:  isMine,
		Label: kind.Label + ": " + strconv.Itoa(count) + service.youSuffix(isMine),
	}
}

func (service *ConversationReactionService) choicesFor(kinds []MessageReaction, mine []string) []MessageReaction {
	choices := []MessageReaction{}
	for _, kind := range kinds {
		choices = append(choices, service.choice(kind, slices.Contains(mine, kind.Emoji)))
	}
	return choices
}

func (service *ConversationReactionService) choice(kind MessageReaction, isMine bool) MessageReaction {
	return MessageReaction{
		Emoji: kind.Emoji,
		Glyph: kind.Glyph,
		Mine:  isMine,
		Label: service.verb(isMine) + " " + kind.Label,
	}
}

func (service *ConversationReactionService) kindOf(name string) MessageReaction {
	return service.kindAt(slices.IndexFunc(service.kinds(), service.named(name)), name)
}

func (service *ConversationReactionService) named(name string) func(MessageReaction) bool {
	return func(kind MessageReaction) bool { return kind.Emoji == name }
}

func (service *ConversationReactionService) kindAt(index int, name string) MessageReaction {
	if index < 0 {
		return MessageReaction{Emoji: name, Glyph: name, Label: name}
	}
	return service.kinds()[index]
}

func (service *ConversationReactionService) verb(isMine bool) string {
	if isMine {
		return "Remove"
	}
	return "Add"
}

func (service *ConversationReactionService) youSuffix(isMine bool) string {
	if isMine {
		return ", including you"
	}
	return ""
}

func (service *ConversationReactionService) empty(reaction MessageReaction) bool {
	return reaction.Count <= 0
}

func (service *ConversationReactionService) order(left MessageReaction, right MessageReaction) int {
	return cmp.Or(cmp.Compare(right.Count, left.Count), strings.Compare(left.Emoji, right.Emoji))
}

func (service *ConversationReactionService) kinds() []MessageReaction {
	return []MessageReaction{
		{Emoji: "thumbs_up", Glyph: "👍", Label: "thumbs up"},
		{Emoji: "thumbs_down", Glyph: "👎", Label: "thumbs down"},
		service.sameLabel("heart", "❤️"),
		service.sameLabel("laugh", "😂"),
		service.sameLabel("surprised", "😮"),
		service.sameLabel("sad", "😢"),
		service.sameLabel("party", "🎉"),
		service.sameLabel("eyes", "👀"),
		{Emoji: "check", Glyph: "✅", Label: "check mark"},
		{Emoji: "pray", Glyph: "🙏", Label: "thank you"},
		service.sameLabel("fire", "🔥"),
		service.sameLabel("rocket", "🚀"),
	}
}

func (service *ConversationReactionService) sameLabel(name string, glyph string) MessageReaction {
	return MessageReaction{Emoji: name, Glyph: glyph, Label: name}
}
