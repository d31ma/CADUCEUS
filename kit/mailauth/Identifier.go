package mailauth

type identifier struct {
	domain string
	mode   string
}

func (item identifier) strict() bool {
	return item.mode == "s"
}
