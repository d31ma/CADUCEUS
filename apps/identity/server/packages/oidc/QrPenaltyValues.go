package oidc

type QrPenaltyValues struct {
	RunMinimum    int
	RunBase       int
	BlockArea     int
	BlockWeight   int
	FinderWeight  int
	QuietWidth    int
	BalanceSteps  int
	BalanceWeight int
	FinderBefore  []int
	FinderAfter   []int
}
