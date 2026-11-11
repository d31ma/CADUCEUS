import "mesail.dev/packages/oidc"

type Admission struct {
	Caller  oidc.Caller
	Refusal error
}
