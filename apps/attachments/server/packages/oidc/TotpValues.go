package oidc

type TotpValues struct {
	SecretBytes   int
	PeriodSeconds int64
	Digits        int
	Modulus       uint32
	CounterBytes  int
	ValueBytes    byte
	OffsetMask    byte
	ValueMask     uint32
}
