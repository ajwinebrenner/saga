package saga_test

import (
	"testing"

	"github.com/ajwinebrenner/saga"
	"github.com/stretchr/testify/assert"
)

func TestBuildEmpty(t *testing.T) {
	t.Run("empty skein", func(t *testing.T) {
		_, err := saga.Weave(&saga.Skein{}, nil)
		assert.ErrorIs(t, err, saga.ErrEmptySkein)
		_, err = saga.Weave(nil, nil)
		assert.ErrorIs(t, err, saga.ErrEmptySkein)
	})

	t.Run("empty scene id", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
				{Id: ""},
			},
		}

		_, err := saga.Weave(&root, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyId)
	})

	t.Run("empty sub-scene id", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id: "a",
					Skein: &saga.Skein{
						EntryScene: "b",
						Scenes: []saga.Scene{
							{Id: "b"},
							{Id: ""},
						},
					},
				},
			},
		}

		_, err := saga.Weave(&root, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyId)
	})

	t.Run("empty thread name", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id: "a",
					Threads: []saga.Thread{
						{Name: "tob", Next: "b"},
					},
				},
				{
					Id: "b",
					Threads: []saga.Thread{
						{Name: "toa", Next: "a"},
						{Name: "", Next: "a"},
					},
				},
			},
		}

		_, err := saga.Weave(&root, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyThread)
	})
}

func TestBuildDuplicates(t *testing.T) {
	t.Run("duplicate scene ids", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
				{Id: "b"},
				{Id: "a"},
			},
		}

		_, err := saga.Weave(&root, nil)
		assert.ErrorContains(t, err, `duplicate IDs: ["a"]`)
	})

	t.Run("duplicates at different depth", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
				{Id: "b", Skein: &saga.Skein{
					EntryScene: "a",
					Scenes: []saga.Scene{
						{Id: "a"},
						{Id: "b"},
					},
				}},
			},
		}

		_, err := saga.Weave(&root, nil)
		assert.NoError(t, err)
	})
}

func TestBuildUnknown(t *testing.T) {
	t.Run("entry scenes", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "b",
			AltEntries: []saga.AltEntry{
				{
					Scene: "c",
					From:  "foo",
				},
			},
			Scenes: []saga.Scene{
				{Id: "a"},
			},
		}

		_, err := saga.Weave(&root, nil)
		assert.ErrorContains(t, err, `unknown IDs: ["b" "c"]`)
	})

	t.Run("outcome scenes", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a", Threads: []saga.Thread{
					{
						Name: "tob",
						Next: "b",
					},
					{
						Name: "toc",
						Next: "c",
					},
				}},
				{Id: "b", Threads: []saga.Thread{
					{
						Name: "toa",
						Next: "a",
						Overrides: []saga.Override{
							{
								Next:      "d",
								Condition: nil,
							},
						},
					},
				}},
			},
		}

		_, err := saga.Weave(&root, nil)
		assert.ErrorContains(t, err, `unknown IDs: ["c" "d"]`)
	})
}

type dummySystem struct {
	s string
}

func TestBuildSystems(t *testing.T) {
	t.Run("invalid system", func(t *testing.T) {
		root := saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
			},
		}

		primitive := "str"
		_, err := saga.Weave(&root, []any{primitive})
		assert.ErrorContains(t, err, "adding system 0")
		_, err = saga.Weave(&root, []any{&primitive})
		assert.ErrorContains(t, err, "adding system 0")

		unnamed := struct{}{}
		_, err = saga.Weave(&root, []any{unnamed})
		assert.ErrorContains(t, err, "adding system 0")
		_, err = saga.Weave(&root, []any{&unnamed})
		assert.ErrorContains(t, err, "adding system 0")

		valid := dummySystem{}
		_, err = saga.Weave(&root, []any{valid})
		assert.ErrorContains(t, err, "adding system 0")
		_, err = saga.Weave(&root, []any{&valid, &valid})
		assert.ErrorContains(t, err, "adding system 1")
	})

	validSkein := func() saga.Skein {
		return saga.Skein{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id:   "a",
					Desc: saga.Dyn(func(sys *dummySystem) string { return sys.s }),
					Threads: []saga.Thread{
						{
							Name:      "tob",
							Next:      "b",
							Desc:      saga.Dyn(func(sys *dummySystem) string { return sys.s }),
							Event:     saga.Dyn(func(sys *dummySystem) string { return sys.s }),
							Condition: saga.Dyn(func(sys *dummySystem) bool { return sys.s == "" }),
							Overrides: []saga.Override{
								{
									Next:      "b",
									Event:     saga.Dyn(func(sys *dummySystem) string { return sys.s }),
									Condition: saga.Dyn(func(sys *dummySystem) bool { return sys.s == "" }),
								},
							},
						},
					},
				},
				{
					Id: "b",
				},
			},
		}
	}

	t.Run("invalid dyn value", func(t *testing.T) {
		invalidDynStr := saga.Dyn(func(sys dummySystem) string { return sys.s })
		invalidDynBool := saga.Dyn(func(sys dummySystem) bool { return sys.s == "" })

		root := validSkein()
		root.Scenes[0].Desc = invalidDynStr
		_, err := saga.Weave(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": desc`)

		root = validSkein()
		root.Scenes[0].Threads[0].Desc = invalidDynStr
		_, err = saga.Weave(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": desc`)

		root = validSkein()
		root.Scenes[0].Threads[0].Event = invalidDynStr
		_, err = saga.Weave(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": event`)

		root = validSkein()
		root.Scenes[0].Threads[0].Condition = invalidDynBool
		_, err = saga.Weave(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": condition`)

		root = validSkein()
		root.Scenes[0].Threads[0].Overrides[0].Event = invalidDynStr
		_, err = saga.Weave(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": override 0: event`)

		root = validSkein()
		root.Scenes[0].Threads[0].Overrides[0].Condition = invalidDynBool
		_, err = saga.Weave(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": override 0: condition`)
	})

	t.Run("missing systems", func(t *testing.T) {
		root := validSkein()

		_, err := saga.Weave(&root, []any{&dummySystem{}})
		assert.NoError(t, err)

		_, err = saga.Weave(&root, nil)
		assert.ErrorContains(t, err, "missing necessary systems")
	})
}
