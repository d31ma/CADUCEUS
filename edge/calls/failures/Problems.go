package failures

import (
	"errors"
	"net/http"
)

type Problems struct{}

func (problems Problems) Unavailable() Problem {
	return Problem{Status: http.StatusInternalServerError, Code: problems.unavailableCode(), Text: "Something went wrong.", Case: 6500}
}

func (problems Problems) MissingToken() Problem {
	return problems.unauthenticated(6501, "The join token is missing.")
}

func (problems Problems) InvalidToken() Problem {
	return problems.unauthenticated(6502, "The join token is not valid.")
}

func (problems Problems) ExpiredToken() Problem {
	return problems.unauthenticated(6503, "The join token has expired.")
}

func (problems Problems) TokenForAnotherCall() Problem {
	return problems.unauthenticated(6504, "The join token is for another call.")
}

func (problems Problems) TokenForAnotherDomain() Problem {
	return problems.unauthenticated(6505, "The join token is for another domain.")
}

func (Problems) CallFull() Problem {
	return Problem{Status: http.StatusConflict, Code: "call_full", Text: "The call is full.", Case: 6506}
}

func (problems Problems) InvalidJson() Problem {
	return problems.invalidRequest(6507, "The request body is not valid JSON.", "")
}

func (Problems) TooLarge() Problem {
	return Problem{Status: http.StatusRequestEntityTooLarge, Code: "too_large", Text: "The request is too large.", Case: 6508}
}

func (problems Problems) InvalidOffer() Problem {
	return problems.invalidRequest(6509, "The offer is not valid.", "sdp")
}

func (problems Problems) InvalidCandidate() Problem {
	return problems.invalidRequest(6510, "A candidate is not valid.", "candidates")
}

func (Problems) NotInCall() Problem {
	return Problem{Status: http.StatusNotFound, Code: "not_found", Text: "This device is not in the call.", Case: 6511}
}

func (problems Problems) KeysUnavailable() Problem {
	return Problem{Status: http.StatusServiceUnavailable, Code: problems.unavailableCode(), Text: "The call cannot be checked now. Try again soon.", Case: 6512}
}

func (problems Problems) All() []Problem {
	return []Problem{
		problems.Unavailable(), problems.MissingToken(), problems.InvalidToken(), problems.ExpiredToken(),
		problems.TokenForAnotherCall(), problems.TokenForAnotherDomain(), problems.CallFull(), problems.InvalidJson(),
		problems.TooLarge(), problems.InvalidOffer(), problems.InvalidCandidate(), problems.NotInCall(),
		problems.KeysUnavailable(),
	}
}

func (problems Problems) From(err error) Problem {
	problem := Problem{}
	found := errors.As(err, &problem)
	return problems.foundOrUnavailable(problem, found)
}

func (problems Problems) foundOrUnavailable(problem Problem, found bool) Problem {
	if !found {
		return problems.Unavailable()
	}
	return problem
}

func (Problems) unauthenticated(number int, text string) Problem {
	return Problem{Status: http.StatusUnauthorized, Code: "unauthenticated", Text: text, Case: number}
}

func (Problems) invalidRequest(number int, text string, field string) Problem {
	return Problem{Status: http.StatusBadRequest, Code: "invalid_request", Text: text, Field: field, Case: number}
}

func (Problems) unavailableCode() string {
	return "unavailable"
}
