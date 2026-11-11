package richtext

type markdownRule struct {
	active bool
	handle func(markdownLine)
}

func (rule markdownRule) isActive() bool {
	return rule.active
}
