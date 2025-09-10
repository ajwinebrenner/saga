package saga_test

import (
	"testing"

	"github.com/ajwinebrenner/saga"
	"github.com/stretchr/testify/assert"
)

func TestBuildEmpty(t *testing.T) {
	t.Run("empty group", func(t *testing.T) {
		_, err := saga.Build(&saga.Group{}, &saga.State{})
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

	t.Run("empty subscene id", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id: "a",
					Group: &saga.Group{
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

	t.Run("empty option name", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{
					Id: "a",
					Options: []saga.Option{
						{Name: "tob", Next: "b"},
					},
				},
				{
					Id: "b",
					Options: []saga.Option{
						{Name: "toa", Next: "a"},
						{Name: "", Next: "a"},
					},
				},
			},
		}

		_, err := saga.Build(&root, nil)
		assert.ErrorIs(t, err, saga.ErrEmptyOption)
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
				{Id: "b", Group: &saga.Group{
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
	t.Run("entry scenes", func(t *testing.T) {
		root := saga.Group{
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

		_, err := saga.Build(&root, &saga.State{})
		assert.ErrorContains(t, err, `unknown IDs: ["b" "c"]`)
	})

	t.Run("outcome scenes", func(t *testing.T) {
		root := saga.Group{
			EntryScene: "a",
			Scenes: []saga.Scene{
				{Id: "a", Options: []saga.Option{
					{
						Name: "tob",
						Next: "b",
					},
					{
						Name: "toc",
						Next: "c",
					},
				}},
				{Id: "b", Options: []saga.Option{
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

		_, err := saga.Build(&root, nil)
		assert.ErrorContains(t, err, `unknown IDs: ["c" "d"]`)
	})
}
