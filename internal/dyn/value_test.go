package dyn_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ajwinebrenner/saga"
	"github.com/ajwinebrenner/saga/internal/dyn"
	"github.com/ajwinebrenner/saga/internal/errs"
	"github.com/ajwinebrenner/saga/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatic(t *testing.T) {
	val := dyn.Static("hello")
	assert.Equal(t, "hello", val.Eval(nil))
	assertSystems(t, val, nil, nil)
}

func TestSingle(t *testing.T) {
	type counter struct {
		num int
	}

	var val saga.DynVal[string]
	coll := system.NewCollection()

	val = dyn.Single(func(c *counter) string {
		return fmt.Sprintf("number is %d", c.num)
	})
	assertSystems(t, val, []reflect.Type{reflect.TypeFor[counter]()}, nil)
	assert.Panics(t, func() { val.Eval(coll) })
	coll.Add(&counter{num: 1})
	assert.Equal(t, "number is 1", val.Eval(coll))

	val = dyn.Single(func(c counter) string {
		return fmt.Sprintf("number is %d", c.num)
	})
	assertSystems(t, val, nil, errs.Static("function argument must be a pointer"))
	assert.Panics(t, func() { val.Eval(coll) })

	val = dyn.Single(func(i *int) string {
		return fmt.Sprintf("number is %d", *i)
	})
	assertSystems(t, val, []reflect.Type{reflect.TypeFor[int]()}, nil)
	assert.Panics(t, func() { val.Eval(coll) })
}

func TestMultiple(t *testing.T) {
	type Counter struct {
		num int
	}

	type Toggle struct {
		on bool
	}

	var val saga.DynVal[string]
	coll := system.NewCollection()

	val = dyn.Multiple(func(c *Counter) string {
		return fmt.Sprintf("number is %d", c.num)
	})
	assertSystems(t, val, nil, errs.Static("function argument must be an anonymous struct"))
	assert.Panics(t, func() { val.Eval(coll) })

	val = dyn.Multiple(func(c Counter) string {
		return fmt.Sprintf("number is %d", c.num)
	})
	assertSystems(t, val, nil, errs.Static("function argument must be an anonymous struct"))
	assert.Panics(t, func() { val.Eval(coll) })

	val = dyn.Multiple(func(s struct {
		C *Counter
		T *Toggle
	}) string {
		return fmt.Sprintf("count %d, toggle %t", s.C.num, s.T.on)
	})
	assertSystems(t, val, []reflect.Type{reflect.TypeFor[Counter](), reflect.TypeFor[Toggle]()}, nil)
	assert.Panics(t, func() { val.Eval(coll) })
	coll.Add(&Counter{num: 1})
	assert.Panics(t, func() { val.Eval(coll) })
	coll.Add(&Toggle{on: true})
	assert.Equal(t, "count 1, toggle true", val.Eval(coll))

	// embedded works just the same
	val = dyn.Multiple(func(s struct {
		*Counter
		*Toggle
	}) string {
		return fmt.Sprintf("count %d, toggle %t", s.num, s.on)
	})
	assertSystems(t, val, []reflect.Type{reflect.TypeFor[Counter](), reflect.TypeFor[Toggle]()}, nil)
	assert.Equal(t, "count 1, toggle true", val.Eval(coll))

	val = dyn.Multiple(func(s struct {
		c *Counter
		Toggle
	}) string {
		return fmt.Sprintf("count %d, toggle %t", s.c.num, s.Toggle.on)
	})
	assertSystems(t, val, []reflect.Type{reflect.TypeFor[Counter]()}, errs.Static("field 0 is unexported\nfield \"Toggle\" must be a pointer"))
	assert.Panics(t, func() { val.Eval(coll) })
}

func assertSystems[T any](t *testing.T, val saga.DynVal[T], sysTypes []reflect.Type, errExpected error) {
	t.Helper()

	res, err := val.Systems()
	assert.Equal(t, sysTypes, res)
	if errExpected == nil {
		assert.Nil(t, err)
	} else {
		require.NotNil(t, err)
		assert.Equal(t, errExpected.Error(), err.Error())
	}
}
