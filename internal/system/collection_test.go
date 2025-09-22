package system_test

import (
	"reflect"
	"testing"

	"github.com/ajwinebrenner/saga/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummySystem struct {
	foo string
}

func TestAdd(t *testing.T) {
	collection := system.NewCollection()

	err := collection.Add(nil)
	assert.ErrorIs(t, err, system.ErrPointer)

	nonStruct := "string"
	err = collection.Add(nonStruct)
	assert.ErrorIs(t, err, system.ErrPointer)
	err = collection.Add(&nonStruct)
	assert.ErrorIs(t, err, system.ErrInvalidType)

	unnamed := struct{}{}
	err = collection.Add(unnamed)
	assert.ErrorIs(t, err, system.ErrPointer)
	err = collection.Add(&unnamed)
	assert.ErrorIs(t, err, system.ErrInvalidType)

	valid := dummySystem{foo: "test"}
	err = collection.Add(valid)
	assert.ErrorIs(t, err, system.ErrPointer)
	err = collection.Add(&valid)
	assert.NoError(t, err)

	err = collection.Add(&dummySystem{foo: "dupe"})
	assert.ErrorContains(t, err, "found duplicate system")
}

func TestGet(t *testing.T) {
	collection := system.NewCollection()
	input := dummySystem{foo: "test"}

	err := collection.Add(&input)
	require.NoError(t, err)

	res, err := collection.Get(reflect.TypeFor[dummySystem]())
	require.NoError(t, err)

	sys, ok := res.(*dummySystem)
	require.True(t, ok, "asserting type as system pointer")
	assert.Equal(t, "test", sys.foo)

	// state is preserved
	sys.foo = "updated"
	assert.Equal(t, "updated", input.foo)

	_, err = collection.Get(reflect.TypeFor[int]())
	assert.ErrorIs(t, err, system.ErrInvalidType)
	_, err = collection.Get(reflect.TypeFor[*dummySystem]())
	assert.ErrorIs(t, err, system.ErrInvalidType)
	_, err = collection.Get(reflect.TypeFor[struct{}]())
	assert.ErrorIs(t, err, system.ErrInvalidType)

	type bar struct{}
	_, err = collection.Get(reflect.TypeFor[bar]())
	assert.ErrorIs(t, err, system.NotFoundError{Key: "github.com/ajwinebrenner/saga/internal/system_test.bar"})
}

func TestMustGet(t *testing.T) {
	collection := system.NewCollection()
	input := dummySystem{foo: "test"}

	err := collection.Add(&input)
	require.NoError(t, err)

	res := collection.MustGet(reflect.TypeFor[dummySystem]())
	sys, ok := res.(*dummySystem)
	require.True(t, ok, "asserting type as system pointer")
	assert.Equal(t, "test", sys.foo)

	sys.foo = "new"
	assert.Equal(t, "new", input.foo)

	assert.Nil(t, collection.MustGet(reflect.TypeFor[int]()))
	assert.Nil(t, collection.MustGet(reflect.TypeFor[*dummySystem]()))
	assert.Nil(t, collection.MustGet(reflect.TypeFor[struct{}]()))

	type other struct{}
	assert.Nil(t, collection.MustGet(reflect.TypeFor[other]()))
}
