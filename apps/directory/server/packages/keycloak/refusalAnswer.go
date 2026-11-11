package keycloak

type refusalAnswer struct {
	ErrorMessage     string `json:"errorMessage"`
	ErrorDescription string `json:"error_description"`
}

func (answer refusalAnswer) message() string {
	if answer.ErrorMessage != "" {
		return answer.ErrorMessage
	}
	return answer.ErrorDescription
}
