package saga_test

import (
	"fmt"
	"testing"

	"github.com/ajwinebrenner/saga"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummySystem struct {
	foo string
}

func TestAddSystem(t *testing.T) {
	state := saga.NewState()

	err := state.AddSystem(nil)
	assert.ErrorIs(t, err, saga.ErrSystemPointer)

	nonStruct := "string"
	err = state.AddSystem(nonStruct)
	assert.ErrorIs(t, err, saga.ErrSystemPointer)
	err = state.AddSystem(&nonStruct)
	assert.ErrorIs(t, err, saga.ErrSystemType)

	unnamed := struct{}{}
	err = state.AddSystem(unnamed)
	assert.ErrorIs(t, err, saga.ErrSystemPointer)
	err = state.AddSystem(&unnamed)
	assert.ErrorIs(t, err, saga.ErrSystemType)

	valid := dummySystem{
		foo: "test",
	}
	err = state.AddSystem(valid)
	assert.ErrorIs(t, err, saga.ErrSystemPointer)
	err = state.AddSystem(&valid)
	assert.Nil(t, err)
}

func TestSystem(t *testing.T) {
	state := saga.NewState()
	valid := dummySystem{
		foo: "test",
	}

	err := state.AddSystem(&valid)
	require.Nil(t, err)

	sys, err := saga.TrySystem[dummySystem](state)
	require.Nil(t, err)
	assert.Equal(t, "test", sys.foo)

	// state is preserved
	sys.foo = "updated"
	sys, err = saga.TrySystem[dummySystem](state)
	assert.Nil(t, err)
	assert.Equal(t, "updated", sys.foo)

	_, err = saga.TrySystem[int](state)
	assert.ErrorIs(t, err, saga.ErrSystemType)
	_, err = saga.TrySystem[*dummySystem](state)
	assert.ErrorIs(t, err, saga.ErrSystemType)
	_, err = saga.TrySystem[struct{}](state)
	assert.ErrorIs(t, err, saga.ErrSystemType)

	type bar struct{}
	_, err = saga.TrySystem[bar](state)
	assert.ErrorIs(t, err, saga.SystemNotFoundError{Key: "bar"})
}

func TestSysFunc(t *testing.T) {
	type foo struct {
		bar int
	}

	type sys struct {
		s string
	}

	state := saga.NewState()
	state.AddSystem(&foo{bar: 5})
	state.AddSystem(&sys{s: "hi"})

	str, err := saga.ExecSysFunc(state, func(s *foo) string {
		return fmt.Sprint(s.bar)
	})
	assert.NoError(t, err)
	assert.Equal(t, "5", str)

	str, err = saga.ExecSysFunc(state, func(s struct {
		Foo *foo
		Sys *sys
	}) string {
		return fmt.Sprintf("num: %d, string: %s", s.Foo.bar, s.Sys.s)
	})
	assert.NoError(t, err)
	assert.Equal(t, "num: 5, string: hi", str)

}
