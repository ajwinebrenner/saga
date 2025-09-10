package saga

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type State struct {
	systems map[string]any
}

func NewState() *State {
	return &State{
		systems: make(map[string]any),
	}
}

type stringError string

func (e stringError) Error() string {
	return string(e)
}

const (
	ErrSystemPointer = stringError("system must be a valid pointer")
	ErrSystemType    = stringError("underlying system type must be a named struct")
)

type SystemNotFoundError struct {
	Key string
}

func (e SystemNotFoundError) Error() string {
	return fmt.Sprintf("system %q not found", e.Key)
}

func (s *State) AddSystem(system any) error {
	if !nonNilPointer(reflect.ValueOf(system)) {
		return ErrSystemPointer
	}

	innerType := reflect.TypeOf(system).Elem()
	if !validSystemType(innerType) {
		return ErrSystemType
	}

	key := systemKey(innerType)
	if _, ok := s.systems[key]; ok {
		return fmt.Errorf("found duplicate system %q", key)
	}

	s.systems[key] = system
	return nil
}

// t is the system type. Returns a pointer to the concrete system
func (s *State) getSystem(t reflect.Type) (any, error) {
	if !validSystemType(t) {
		return nil, ErrSystemType
	}

	out, ok := s.systems[systemKey(t)]
	if !ok {
		return nil, SystemNotFoundError{Key: t.Name()}
	}

	return out, nil
}

func (s *State) mustGetSystem(t reflect.Type) any {
	return s.systems[systemKey(t)]
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

func TrySystem[T any](state *State) (*T, error) {
	sys, err := state.getSystem(reflect.TypeOf([0]T{}).Elem())
	if err != nil {
		return nil, err
	}

	res, ok := sys.(*T)
	if !ok {
		// should be unreachable state
		return nil, fmt.Errorf("found system with mismatching type: %T", sys)
	}

	return res, nil
}

// assumes system type is valid and is present
func System[T any](state *State) *T {
	sys := state.mustGetSystem(reflect.TypeOf([0]T{}).Elem())
	return sys.(*T)
}

func ExecSysFunc[I, R any](state *State, f func(I) R) (R, error) {
	var resEmpty R
	inType := reflect.TypeOf([0]I{}).Elem()

	// anon struct of system pointers
	if inType.Kind() == reflect.Struct && inType.Name() == "" {
		var input I

		inValue := reflect.ValueOf(&input).Elem()
		for i := range inValue.NumField() {
			field := inValue.Field(i)
			if field.Kind() != reflect.Pointer {
				return resEmpty, ErrSystemPointer
			}
			if !field.CanSet() {
				return resEmpty, fmt.Errorf("field %d is not exported", i)
			}

			sys, err := state.getSystem(field.Type().Elem())
			if err != nil {
				return resEmpty, err
			}

			field.Set(reflect.ValueOf(sys))
		}

		return f(input), nil
	}

	if inType.Kind() != reflect.Pointer {
		return resEmpty, errors.New("function argument must be system pointer or anonymous struct of system pointers")
	}

	out, err := state.getSystem(inType.Elem())
	if err != nil {
		return resEmpty, err
	}

	sys, ok := out.(I)
	if !ok {
		return resEmpty, fmt.Errorf("found mismatching system")
	}

	return f(sys), nil
}

type static[T any] struct {
	v T
}

func (s static[T]) eval(_ *State) T {
	return s.v
}

func (s static[T]) deps() dynDeps {
	return dynDeps{}
}

func Static[T any](v T) static[T] {
	return static[T]{v: v}
}

type single[I, R any] struct {
	f func(*I) R
}

func (s single[I, R]) eval(state *State) R {
	return s.f(System[I](state))
}

func (s single[I, R]) deps() dynDeps {
	return dynDeps{
		systems: []reflect.Type{
			reflect.TypeOf([0]I{}).Elem(),
		},
	}
}

type collection[I, R any] struct {
	f func(I) R
}

type dyn[T any] interface {
	eval(*State) T
	deps() dynDeps
}

type dynDeps struct {
	systems []reflect.Type
}
