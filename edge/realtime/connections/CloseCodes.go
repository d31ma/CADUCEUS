package connections

type CloseCodes struct{}

func (CloseCodes) Normal() int          { return 1000 }
func (CloseCodes) GoingAway() int       { return 1001 }
func (CloseCodes) UnsupportedData() int { return 1003 }
func (CloseCodes) Broken() int          { return 1006 }
func (CloseCodes) MessageTooBig() int   { return 1009 }
func (CloseCodes) InternalError() int   { return 1011 }
func (CloseCodes) TryAgainLater() int   { return 1013 }
func (CloseCodes) FirstPrivate() int    { return 4000 }
func (CloseCodes) LastPrivate() int     { return 4999 }
func (CloseCodes) Unauthorized() int    { return 4401 }
