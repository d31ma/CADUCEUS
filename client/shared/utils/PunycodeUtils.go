import "strings"

// PunycodeUtils writes the ASCII form of an internationalized host label
// (RFC 3492), so hosts read the way the browser URL parser shows them.
type PunycodeUtils struct{}

func (PunycodeUtils) Host(host string) string {
	return PunycodeUtils{}.host(host, ".")
}

func (PunycodeUtils) host(host string, dot string) string {
	encoded := []string{}
	for _, label := range strings.Split(host, dot) {
		encoded = append(encoded, PunycodeUtils{}.label(label))
	}
	return strings.Join(encoded, dot)
}

func (PunycodeUtils) label(label string) string {
	if (PunycodeUtils{}).isAscii(label) {
		return label
	}
	return "xn--" + PunycodeUtils{}.encode([]rune(label))
}

func (PunycodeUtils) firstCodePoint() int { return 128 }

func (PunycodeUtils) isAscii(text string) bool {
	return strings.IndexFunc(text, func(character rune) bool { return character >= rune(PunycodeUtils{}.firstCodePoint()) }) < 0
}

func (PunycodeUtils) firstBias() int { return 72 }

func (PunycodeUtils) encode(input []rune) string {
	basic := PunycodeUtils{}.basicCharacters(input)
	state := &PunycodeState{CodePoint: PunycodeUtils{}.firstCodePoint(), Bias: PunycodeUtils{}.firstBias(), Handled: len(basic), Basic: len(basic), Output: PunycodeUtils{}.basicOutput(basic)}
	for state.Handled < len(input) {
		PunycodeUtils{}.round(input, state)
	}
	return state.Output
}

func (PunycodeUtils) basicCharacters(input []rune) []rune {
	basic := []rune{}
	for _, character := range input {
		basic = append(basic, PunycodeUtils{}.basicCharacter(character)...)
	}
	return basic
}

func (PunycodeUtils) basicCharacter(character rune) []rune {
	if character < rune(PunycodeUtils{}.firstCodePoint()) {
		return []rune{character}
	}
	return []rune{}
}

func (PunycodeUtils) basicOutput(basic []rune) string {
	if len(basic) == 0 {
		return ""
	}
	return string(basic) + "-"
}

func (PunycodeUtils) round(input []rune, state *PunycodeState) {
	next := PunycodeUtils{}.nextCodePoint(input, state.CodePoint)
	state.Delta += (next - state.CodePoint) * (state.Handled + 1)
	state.CodePoint = next
	for _, character := range input {
		PunycodeUtils{}.visit(int(character), state)
	}
	state.Delta++
	state.CodePoint++
}

func (PunycodeUtils) nextCodePoint(input []rune, floor int) int {
	next := int(^uint(0) >> 1)
	for _, character := range input {
		next = PunycodeUtils{}.lower(next, int(character), floor)
	}
	return next
}

func (PunycodeUtils) lower(current int, candidate int, floor int) int {
	if candidate >= floor && candidate < current {
		return candidate
	}
	return current
}

func (PunycodeUtils) visit(character int, state *PunycodeState) {
	switch {
	case character < state.CodePoint:
		state.Delta++
	case character == state.CodePoint:
		PunycodeUtils{}.emit(state)
	}
}

func (PunycodeUtils) base() int { return 36 }

func (PunycodeUtils) emit(state *PunycodeState) {
	quotient := state.Delta
	for step := (PunycodeUtils{}).base(); quotient >= (PunycodeUtils{}).threshold(step, state.Bias); step += (PunycodeUtils{}).base() {
		threshold := PunycodeUtils{}.threshold(step, state.Bias)
		state.Output += PunycodeUtils{}.digit(threshold + (quotient-threshold)%(PunycodeUtils{}.base()-threshold))
		quotient = (quotient - threshold) / (PunycodeUtils{}.base() - threshold)
	}
	state.Output += PunycodeUtils{}.digit(quotient)
	state.Bias = PunycodeUtils{}.adapt(state.Delta, state.Handled+1, state.Handled == state.Basic)
	state.Delta = 0
	state.Handled++
}

func (PunycodeUtils) smallest() int { return 1 }

func (PunycodeUtils) largest() int { return 26 }

func (PunycodeUtils) threshold(step int, bias int) int {
	switch {
	case step <= bias:
		return PunycodeUtils{}.smallest()
	case step >= bias+PunycodeUtils{}.largest():
		return PunycodeUtils{}.largest()
	}
	return step - bias
}

func (PunycodeUtils) digit(value int) string {
	return string("abcdefghijklmnopqrstuvwxyz0123456789"[value])
}

func (PunycodeUtils) skew() int { return 38 }

func (PunycodeUtils) half() int { return 2 }

func (PunycodeUtils) adapt(delta int, points int, first bool) int {
	scaled := PunycodeUtils{}.scaled(delta, first) + PunycodeUtils{}.scaled(delta, first)/points
	steps := 0
	limit := ((PunycodeUtils{}.base() - PunycodeUtils{}.smallest()) * PunycodeUtils{}.largest()) / PunycodeUtils{}.half()
	for ; scaled > limit; steps += (PunycodeUtils{}).base() {
		scaled /= PunycodeUtils{}.base() - PunycodeUtils{}.smallest()
	}
	return steps + (PunycodeUtils{}.base()-PunycodeUtils{}.smallest()+1)*scaled/(scaled+PunycodeUtils{}.skew())
}

func (PunycodeUtils) damp() int { return 700 }

func (PunycodeUtils) scaled(delta int, first bool) int {
	if first {
		return delta / PunycodeUtils{}.damp()
	}
	return delta / PunycodeUtils{}.half()
}

