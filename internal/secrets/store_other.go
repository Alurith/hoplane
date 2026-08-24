//go:build !linux

package secrets

import "context"

type unavailableStore struct{}

func NewDefaultStore() Store {
	return unavailableStore{}
}

func (unavailableStore) Lookup(context.Context, Ref) ([]byte, error) {
	return nil, ErrUnavailable
}

func (unavailableStore) Store(context.Context, Ref, []byte) error {
	return ErrUnavailable
}

func (unavailableStore) Delete(context.Context, Ref) error {
	return ErrUnavailable
}

func (unavailableStore) Has(context.Context, Ref) (bool, error) {
	return false, ErrUnavailable
}
