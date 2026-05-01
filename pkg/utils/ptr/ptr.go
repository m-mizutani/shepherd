// Package ptr collects tiny generic helpers around pointers.
//
// The OpenAPI-generated response structs in this codebase use *T for any
// optional field, so the controller has to hand out a pointer when the
// domain value is non-empty and nil otherwise. Doing that inline turns
// every assignment into three lines (`if v != "" { s := v; r.X = &s }`)
// even though the intent fits on one. This package keeps the boilerplate
// in one place.
package ptr

// NonZero returns a pointer to v when v is not the zero value of T,
// and nil otherwise. Useful for filling optional response fields whose
// presence semantics are "present iff non-empty".
func NonZero[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// Of returns a pointer to a copy of v unconditionally. Use it when the
// caller has already decided the value should appear in the response
// (e.g. inside a conditional that picked a non-default code path) and
// only needs an addressable copy.
func Of[T any](v T) *T {
	return &v
}
