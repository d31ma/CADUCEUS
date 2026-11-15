package main

type ListedNumber struct {
	Number  int
	App     string
	Line    int
	Retired bool
}

type listedKey struct {
	Number  int
	Retired bool
}

func (listed ListedNumber) SameNumber(other ListedNumber) bool {
	return listed.Number == other.Number
}

func (listed ListedNumber) SameRetiredNumber(other ListedNumber) bool {
	return listed.key() == other.key()
}

func (listed ListedNumber) key() listedKey {
	return listedKey{Number: listed.Number, Retired: listed.Retired}
}
