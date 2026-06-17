package saga_test

import (
	"testing"

	"github.com/ajwinebrenner/saga"
	"github.com/stretchr/testify/assert"
)

func TestBuildEmpty(t *testing.T) {
	t.Run("empty group", func(t *testing.T) {
		_, err := saga.Build(&saga.Group{}, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyGroup)
		_, err = saga.Build(nil, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyGroup)
	})

	t.Run("empty scene id", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
				{Id: ""},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyId)
	})

	t.Run("empty sub-scene id", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id: "a",
					SubScenes: &saga.Group{
						EntryScene: "b",
						Scenes: []saga.Scene{
							{Id: "b"},
							{Id: ""},
						},
					},
				},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyId)
	})

	t.Run("empty choice name", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id: "a",
					Choices: []saga.Choice{
						{Name: "tob", Route: saga.Route{To: "b"}},
					},
				},
				{
					Id: "b",
					Choices: []saga.Choice{
						{Name: "toa", Route: saga.Route{To: "a"}},
						{Name: "", Route: saga.Route{To: "a"}},
					},
				},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyChoice)
	})
}

func TestBuildDuplicates(t *testing.T) {
	t.Run("duplicate scene ids", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
				{Id: "b"},
				{Id: "a"},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorContains(t, err, `duplicate IDs: ["a"]`)
	})

	t.Run("duplicates at different depth", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
				{Id: "b", SubScenes: &saga.Group{
					EntryScene: "a",
					Scenes: []saga.Scene{
						{Id: "a"},
						{Id: "b"},
					},
				}},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.NoError(t, err)
	})
}

func TestBuildUnknown(t *testing.T) {
	t.Run("entry scene", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "b",
			Scenes: []saga.Scene{
				{Id: "a"},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorContains(t, err, `unknown IDs: ["b"]`)
	})

	t.Run("choice scenes", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a", Choices: []saga.Choice{
					{
						Name:  "tob",
						Route: saga.Route{To: "b"},
					},
					{
						Name:  "toc",
						Route: saga.Route{To: "c"},
					},
				}},
				{Id: "b", Choices: []saga.Choice{
					{
						Name:  "toa",
						Route: saga.Route{To: "a"},
						Overrides: []saga.Route{
							{
								To:     "d",
								Active: nil,
							},
						},
					},
				}},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorContains(t, err, `unknown IDs: ["c" "d"]`)
	})

	t.Run("reroute scenes", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a", Reroutes: []saga.Route{
					{To: "b", Active: saga.Just(true)},
					{To: "c", Active: nil},
				}},
				{Id: "b", Reroutes: []saga.Route{
					{To: "d", Active: saga.Just(true)},
				}},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorContains(t, err, `unknown IDs: ["c" "d"]`)
	})
}

type dummySystem struct {
	s string
}

func TestBuildSystems(t *testing.T) {
	t.Run("invalid system", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a"},
			},
		}

		primitive := "str"
		_, err := saga.Build(&root, []any{primitive})
		assert.ErrorContains(t, err, "adding system 0")
		_, err = saga.Build(&root, []any{&primitive})
		assert.ErrorContains(t, err, "adding system 0")

		unnamed := struct{}{}
		_, err = saga.Build(&root, []any{unnamed})
		assert.ErrorContains(t, err, "adding system 0")
		_, err = saga.Build(&root, []any{&unnamed})
		assert.ErrorContains(t, err, "adding system 0")

		valid := dummySystem{}
		_, err = saga.Build(&root, []any{valid})
		assert.ErrorContains(t, err, "adding system 0")
		_, err = saga.Build(&root, []any{&valid, &valid})
		assert.ErrorContains(t, err, "adding system 1")
	})

	validGroup := func() saga.Group {
		return saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id:   "a",
					Desc: saga.Dyn(func(sys *dummySystem) string { return sys.s }),
					Choices: []saga.Choice{
						{
							Name: "tob",
							Desc: saga.Dyn(func(sys *dummySystem) string { return sys.s }),
							Route: saga.Route{
								To:     "b",
								Event:  saga.Dyn(func(sys *dummySystem) string { return sys.s }),
								Active: saga.Dyn(func(sys *dummySystem) bool { return sys.s == "" }),
							},
							Overrides: []saga.Route{
								{
									To:     "b",
									Event:  saga.Dyn(func(sys *dummySystem) string { return sys.s }),
									Active: saga.Dyn(func(sys *dummySystem) bool { return sys.s == "" }),
								},
							},
						},
					},
				},
				{
					Id: "b",
					Reroutes: []saga.Route{
						{
							To:     "a",
							Event:  saga.Dyn(func(sys *dummySystem) string { return sys.s }),
							Active: saga.Dyn(func(sys *dummySystem) bool { return sys.s == "" }),
						},
					},
				},
			},
		}
	}

	t.Run("invalid dyn value", func(t *testing.T) {
		invalidDynStr := saga.Dyn(func(sys dummySystem) string { return sys.s })
		invalidDynBool := saga.Dyn(func(sys dummySystem) bool { return sys.s == "" })

		root := validGroup()
		root.Scenes[0].Desc = invalidDynStr
		_, err := saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": desc`)

		root = validGroup()
		root.Scenes[0].Choices[0].Desc = invalidDynStr
		_, err = saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": desc`)

		root = validGroup()
		root.Scenes[0].Choices[0].Route.Event = invalidDynStr
		_, err = saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": route: event`)

		root = validGroup()
		root.Scenes[0].Choices[0].Route.Active = invalidDynBool
		_, err = saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": route: active`)

		root = validGroup()
		root.Scenes[0].Choices[0].Overrides[0].Event = invalidDynStr
		_, err = saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": override[0]: event`)

		root = validGroup()
		root.Scenes[0].Choices[0].Overrides[0].Active = invalidDynBool
		_, err = saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"a": "tob": override[0]: active`)

		root = validGroup()
		root.Scenes[1].Reroutes[0].Event = invalidDynStr
		_, err = saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"b": reroute[0]: event`)

		root = validGroup()
		root.Scenes[1].Reroutes[0].Active = invalidDynBool
		_, err = saga.Build(&root, []any{&dummySystem{}})
		assert.ErrorContains(t, err, `"b": reroute[0]: active`)
	})

	t.Run("missing systems", func(t *testing.T) {
		root := validGroup()

		_, err := saga.Build(&root, []any{&dummySystem{}})
		assert.NoError(t, err)

		_, err = saga.Build(&root, nil)
		assert.ErrorContains(t, err, "missing necessary systems")
	})
}
