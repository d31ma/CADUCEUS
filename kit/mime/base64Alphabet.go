package mime

type base64Alphabet struct{}

func (base64Alphabet) Letters() string {
	return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
}
