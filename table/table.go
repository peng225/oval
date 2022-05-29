package table

type Object struct {
	key        string
	size       int
	writeCount int
	worker     string
	bucket     string
}

type ObjectTable struct {
	numObj  int
	entries map[string]Object
}
