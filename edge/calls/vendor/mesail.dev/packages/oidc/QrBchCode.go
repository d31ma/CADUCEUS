package oidc

type QrBchCode struct{}

func (code QrBchCode) encode(data int, degree int, generator int) int {
	return data<<degree | code.remainder(data, degree, generator)
}

func (QrBchCode) remainder(data int, degree int, generator int) int {
	result := data
	for range degree {
		result = (result << 1) ^ ((result >> (degree - 1)) * generator)
	}
	return result
}
