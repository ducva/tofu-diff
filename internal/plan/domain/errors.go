package domain

import (
	"errors"
	"fmt"
)

var ErrMissingResourceAddress = errors.New("resource change is missing an address")

type DuplicateResourceAddressError struct{ Address string }

func (e DuplicateResourceAddressError) Error() string {
	return fmt.Sprintf("duplicate resource address %q", e.Address)
}

type InvalidResourceChangeError struct {
	Address string
	Err     error
}

func (e InvalidResourceChangeError) Error() string {
	return fmt.Sprintf("resource %q: %v", e.Address, e.Err)
}

func (e InvalidResourceChangeError) Unwrap() error { return e.Err }
