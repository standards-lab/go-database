package postgres

import "strconv"

type dialect struct{}

func (dialect) Name() string {
	return string(Provider)
}

func (dialect) Placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}

func (dialect) MapError(err error) error {
	return err
}
