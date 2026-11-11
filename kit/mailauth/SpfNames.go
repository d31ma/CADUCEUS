package mailauth

type spfNames struct{}

func (spfNames) all() string      { return "all" }
func (spfNames) include() string  { return "include" }
func (spfNames) address() string  { return "a" }
func (spfNames) exchange() string { return "mx" }
func (spfNames) ip4() string      { return "ip4" }
func (spfNames) ip6() string      { return "ip6" }
func (spfNames) exists() string   { return "exists" }
func (spfNames) pointer() string  { return "ptr" }
func (spfNames) redirect() string { return "redirect" }
func (spfNames) explain() string  { return "exp" }
func (spfNames) version() string  { return "v=spf1" }
