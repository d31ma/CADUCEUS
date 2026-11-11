package mime

import "strings"

type charsets struct{}

func (sets charsets) Decode(name string, content []byte) string {
	decode, known := sets.decoders()[strings.ToLower(strings.TrimSpace(name))]
	return sets.pick(decode, known)(content)
}

func (sets charsets) Valid(text string) string {
	return strings.ToValidUTF8(text, sets.replacement())
}

func (charsets) replacement() string {
	return "�"
}

func (sets charsets) decoders() map[string]func([]byte) string {
	return map[string]func([]byte) string{
		"utf-8":        sets.utf8,
		"utf8":         sets.utf8,
		"us-ascii":     sets.utf8,
		"ascii":        sets.utf8,
		"iso-8859-1":   sets.latin1,
		"iso8859-1":    sets.latin1,
		"iso_8859-1":   sets.latin1,
		"latin1":       sets.latin1,
		"l1":           sets.latin1,
		"windows-1252": sets.windows1252,
		"cp1252":       sets.windows1252,
	}
}

func (sets charsets) pick(decode func([]byte) string, known bool) func([]byte) string {
	if known {
		return decode
	}
	return sets.utf8
}

func (sets charsets) utf8(content []byte) string {
	return sets.Valid(string(content))
}

func (charsets) latin1(content []byte) string {
	runes := make([]rune, len(content))
	for index, value := range content {
		runes[index] = rune(value)
	}
	return string(runes)
}

func (sets charsets) windows1252(content []byte) string {
	table := sets.windowsTable()
	runes := make([]rune, len(content))
	for index, value := range content {
		runes[index] = sets.windowsRune(table, value)
	}
	return string(runes)
}

func (charsets) windowsRune(table map[byte]rune, value byte) rune {
	mapped, found := table[value]
	if found {
		return mapped
	}
	return rune(value)
}

func (charsets) windowsTable() map[byte]rune {
	return map[byte]rune{
		0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…',
		0x86: '†', 0x87: '‡', 0x88: 'ˆ', 0x89: '‰', 0x8a: 'Š',
		0x8b: '‹', 0x8c: 'Œ', 0x8e: 'Ž', 0x91: '‘', 0x92: '’',
		0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
		0x98: '˜', 0x99: '™', 0x9a: 'š', 0x9b: '›', 0x9c: 'œ',
		0x9e: 'ž', 0x9f: 'Ÿ',
	}
}
