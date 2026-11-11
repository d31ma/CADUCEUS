package mailauth

type dkimAlgorithms struct{}

func (dkimAlgorithms) rsa() string     { return "rsa" }
func (dkimAlgorithms) ed25519() string { return "ed25519" }
func (dkimAlgorithms) sha1() string    { return "rsa-sha1" }

func (algorithms dkimAlgorithms) kind(algorithm string) (string, error) {
	kind, found := algorithms.kinds()[algorithm]
	switch {
	case found:
		return kind, nil
	case algorithm == algorithms.sha1():
		return "", failure{result: Outcomes{}.Fail()}
	default:
		return "", failure{result: Outcomes{}.PermError()}
	}
}

func (algorithms dkimAlgorithms) kinds() map[string]string {
	return map[string]string{"rsa-sha256": algorithms.rsa(), "ed25519-sha256": algorithms.ed25519()}
}
