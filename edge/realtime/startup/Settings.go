package startup

type Settings struct {
	PublicAddress     string
	ManagementAddress string
	BackendUrl        string
	Secret            string
	Origins           []string
}
