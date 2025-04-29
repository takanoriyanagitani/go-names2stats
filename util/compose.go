package util

import (
	mi "github.com/takanoriyanagitani/go-names2stats"
)

func ComposeErr[T, U, V any](
	f func(T) (U, error),
	g func(U) (V, error),
) func(T) (V, error) {
	return mi.ComposeErr(f, g)
}

func Compose[T, U, V any](
	f func(T) U,
	g func(U) V,
) func(T) V {
	return mi.Compose(f, g)
}
