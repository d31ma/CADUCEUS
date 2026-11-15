package login

type People interface {
	Person(personId string) (Person, error)
	EndSessions(personId string) error
}
