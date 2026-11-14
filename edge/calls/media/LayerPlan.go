package media

type LayerPlan struct{}

func (plan LayerPlan) Choose(bitrate int, count int) []int {
	return plan.chosen(bitrate-count*plan.costs()[0], make([]int, count))
}

func (plan LayerPlan) chosen(left int, qualities []int) []int {
	for index := range qualities {
		qualities[index] = max(plan.affordable(left)-1, 0)
		left -= plan.extra(qualities[index])
	}
	return qualities
}

func (plan LayerPlan) affordable(left int) int {
	count := 0
	for quality := range plan.costs() {
		count += plan.fits(plan.extra(quality), left)
	}
	return count
}

func (LayerPlan) fits(extra int, left int) int {
	if extra > left {
		return 0
	}
	return 1
}

func (plan LayerPlan) extra(quality int) int {
	return plan.costs()[quality] - plan.costs()[0]
}

func (LayerPlan) costs() []int {
	return []int{150_000, 500_000, 1_500_000}
}
