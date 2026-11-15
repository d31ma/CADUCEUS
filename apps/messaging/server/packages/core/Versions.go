package core

type Versions struct{}

func (Versions) First() int {
	return 1
}

func (versions Versions) Current(message Message) int {
	return max(message.Version, versions.First())
}

func (versions Versions) Next(message Message) int {
	return versions.Current(message) + 1
}
