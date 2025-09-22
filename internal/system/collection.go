package system

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ajwinebrenner/saga/internal/errs"
)

type Collection struct {
	store map[string]any
}

func NewCollection() *Collection {
	return &Collection{
		store: make(map[string]any),
	}
}

// `system` should be a pointer to a named struct.
func (c *Collection) Add(system any) error {
	if !nonNilPointer(reflect.ValueOf(system)) {
		return ErrPointer
	}

	innerType := reflect.TypeOf(system).Elem()
	if !validSystemType(innerType) {
		return ErrInvalidType
	}

	key := systemKey(innerType)
	if _, ok := c.store[key]; ok {
		return fmt.Errorf("found duplicate system %q", key)
	}

	c.store[key] = system
	return nil
}

// Returns a pointer to the system where `t` is the underlying type.
func (c *Collection) Get(t reflect.Type) (any, error) {
	if !validSystemType(t) {
		return nil, ErrInvalidType
	}

	out, ok := c.store[systemKey(t)]
	if !ok {
		return nil, NotFoundError{Key: systemKey(t)}
	}

	return out, nil
}

// Returns a pointer to the system assuming `t` is a valid system type.
// Method will return nil if `t` is not a valid type or is not found.
func (c *Collection) MustGet(t reflect.Type) any {
	return c.store[systemKey(t)]
}

func nonNilPointer(v reflect.Value) bool {
	return v.Kind() == reflect.Pointer && !v.IsNil()
}

func validSystemType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.Name() != ""
}

func systemKey(t reflect.Type) string {
	var sb strings.Builder
	if pkg := t.PkgPath(); len(pkg) > 0 {
		sb.WriteString(pkg)
		sb.WriteByte('.')
	}

	sb.WriteString(t.Name())
	return sb.String()
}

const (
	ErrPointer     = errs.Static("system must be a valid pointer")
	ErrInvalidType = errs.Static("underlying system type must be a named struct")
)

type NotFoundError struct {
	Key string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("system %q not found", e.Key)
}
