package oidc

type QrGaloisField struct{}

func (QrGaloisField) primitive() int {
	return 0x11D
}

func (QrGaloisField) elementBits() int {
	return 8
}

func (QrGaloisField) generator() int {
	return 2
}

func (field QrGaloisField) multiply(left int, right int) int {
	result := 0
	for shift := field.elementBits() - 1; shift >= 0; shift-- {
		result = field.reduce(result<<1) ^ (left * ((right >> shift) & 1))
	}
	return result
}

func (field QrGaloisField) reduce(value int) int {
	return value ^ (field.primitive() * (value >> field.elementBits()))
}
