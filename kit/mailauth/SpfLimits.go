package mailauth

type spfLimits struct{}

func (spfLimits) lookups() int   { return 10 }
func (spfLimits) voids() int     { return 2 }
func (spfLimits) exchanges() int { return 10 }
func (spfLimits) bits4() int     { return 32 }
func (spfLimits) bits6() int     { return 128 }
