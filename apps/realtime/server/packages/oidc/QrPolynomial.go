package oidc

import "slices"

type QrPolynomial struct{}

func (polynomial QrPolynomial) generator(degree int) []int {
	field := QrGaloisField{}
	result := []int{1}
	root := 1
	for range degree {
		result = polynomial.multiplyByRoot(result, root)
		root = field.multiply(root, field.generator())
	}
	return result
}

func (polynomial QrPolynomial) multiplyByRoot(coefficients []int, root int) []int {
	shifted := append(slices.Clone(coefficients), 0)
	scaled := append([]int{0}, polynomial.scale(coefficients, root)...)
	return polynomial.add(shifted, scaled)
}

func (polynomial QrPolynomial) remainder(data []byte, degree int) []byte {
	divisor := polynomial.generator(degree)[1:]
	result := make([]int, degree)
	for _, value := range data {
		result = polynomial.divideStep(result, int(value), divisor)
	}
	return polynomial.toBytes(result)
}

func (polynomial QrPolynomial) divideStep(remainder []int, value int, divisor []int) []int {
	factor := value ^ remainder[0]
	shifted := append(slices.Clone(remainder[1:]), 0)
	return polynomial.add(shifted, polynomial.scale(divisor, factor))
}

func (QrPolynomial) scale(coefficients []int, factor int) []int {
	field := QrGaloisField{}
	result := make([]int, len(coefficients))
	for index, coefficient := range coefficients {
		result[index] = field.multiply(coefficient, factor)
	}
	return result
}

func (QrPolynomial) add(left []int, right []int) []int {
	result := make([]int, len(left))
	for index := range left {
		result[index] = left[index] ^ right[index]
	}
	return result
}

func (QrPolynomial) toBytes(values []int) []byte {
	result := make([]byte, len(values))
	for index, value := range values {
		result[index] = byte(value)
	}
	return result
}
