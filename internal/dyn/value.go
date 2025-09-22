package dyn

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/ajwinebrenner/saga/internal/errs"
	"github.com/ajwinebrenner/saga/internal/system"
)

func Static[V any](v V) static[V] {
	return static[V]{v: v}
}

type static[V any] struct{ v V }

func (s static[V]) Eval(_ *system.Collection) V {
	return s.v
}

func (s static[V]) Systems() ([]reflect.Type, error) {
	return nil, nil
}

func Single[S, V any](f func(S) V) single[S, V] {
	return single[S, V]{f: f}
}

type single[S, V any] struct {
	f func(S) V
}

func (s single[S, V]) Eval(collection *system.Collection) V {
	sys := collection.MustGet(reflect.TypeFor[S]().Elem())
	return s.f(sys.(S))
}

func (s single[S, V]) Systems() ([]reflect.Type, error) {
	sysType := reflect.TypeFor[S]()

	if sysType.Kind() != reflect.Pointer {
		return nil, errs.Static("function argument must be a pointer")
	}

	return []reflect.Type{sysType.Elem()}, nil

}

func Multiple[M, V any](f func(M) V) multiple[M, V] {
	return multiple[M, V]{f: f}
}

// Assumption that M is an anon struct with exported fields of kind pointer.
type multiple[M, V any] struct {
	f func(M) V
}

func (m multiple[M, V]) Eval(collection *system.Collection) V {
	var input M

	inValue := reflect.ValueOf(&input).Elem()
	for i := range inValue.NumField() {
		field := inValue.Field(i)

		sys := collection.MustGet(field.Type().Elem())
		field.Set(reflect.ValueOf(sys))
	}

	return m.f(input)
}

func (m multiple[M, V]) Systems() ([]reflect.Type, error) {
	inType := reflect.TypeFor[M]()
	if !anonStruct(inType) {
		return nil, errs.Static("function argument must be an anonymous struct")
	}

	var problems []error
	inSize := inType.NumField()
	sysTypes := make([]reflect.Type, 0, inSize)

	for i := range inSize {
		field := inType.Field(i)
		if !field.IsExported() {
			problems = append(problems, fmt.Errorf("field %d is unexported", i))
		}

		if field.Type.Kind() == reflect.Pointer {
			sysTypes = append(sysTypes, field.Type.Elem())
		} else {
			problems = append(problems, fmt.Errorf("field %q must be a pointer", field.Name))
		}
	}

	return sysTypes, errors.Join(problems...)
}

func MultipleArg[I any]() bool {
	inType := reflect.TypeFor[I]()
	return anonStruct(inType)
}

func anonStruct(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.Name() == ""
}
