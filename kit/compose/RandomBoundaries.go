package compose

import "crypto/rand"

type RandomBoundaries struct{}

func (RandomBoundaries) Next() string {
	return "mesail-" + rand.Text()
}
