package compose

type limits struct{}

func (limits) LineEnd() string          { return "\r\n" }
func (limits) FoldWidth() int           { return 78 }
func (limits) MaxReferences() int       { return 20 }
func (limits) Base64LineLength() int    { return 76 }
func (limits) UnsafeCharacters() string { return "\r\n" }
