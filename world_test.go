package saga

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorld(t *testing.T) {
	root := testGroup()
	state := testState()
	world, err := Build(&root, state)
	require.NoError(t, err)

	type exp struct {
		event  string
		prompt []string
		opts   []ValidOption
	}

	for _, step := range []struct {
		next     string
		preFunc  func()
		expected exp
	}{
		{
			next: "tob",
			expected: exp{
				event:  "going to b",
				prompt: nil,
				opts: []ValidOption{{
					Name:  "toa",
					Desc:  "",
					Depth: 0,
				}},
			},
		},
		{
			next: "toa",
			expected: exp{
				event:  "",
				prompt: []string{"start"},
				opts: []ValidOption{
					{
						Name:  "tob",
						Desc:  "go to b",
						Depth: 0,
					},
					{
						Name:  "toc",
						Depth: 0,
					},
				},
			},
		},
		{
			next: "toc",
			expected: exp{
				event:  "",
				prompt: []string{"count: 0", "subscene"},
				opts: []ValidOption{
					{
						Name:  "to2",
						Desc:  "go to 2",
						Depth: 1,
					},
				},
			},
		},
		{
			next: "to2",
			expected: exp{
				event:  "going to 2",
				prompt: nil,
				opts: []ValidOption{
					{
						Name:  "long3",
						Depth: 1,
					},
					{
						Name:  "to3",
						Depth: 1,
					},
					{
						Name:  "wait",
						Depth: 1,
					},
				},
			},
		},
		{
			next: "wait",
			preFunc: func() {
				System[testSystem](state).count++
			},
			expected: exp{
				event:  "",
				prompt: []string{"count: 1"},
				opts: []ValidOption{
					{
						Name:  "long3",
						Depth: 1,
					},
					{
						Name:  "to3",
						Depth: 1,
					},
					{
						Name:  "wait",
						Depth: 1,
					},
				},
			},
		},
		{
			next: "long3",
			preFunc: func() {
				System[testSystem](state).count++
			},
			expected: exp{
				event:  "taking the long way to 3",
				prompt: []string{"count: 2", "you are at 3"},
				opts: []ValidOption{
					{
						Name:  "to1",
						Depth: 1,
					},
				},
			},
		},
		{
			next: "to1",
			expected: exp{
				event:  "",
				prompt: []string{"subscene"},
				opts: []ValidOption{
					{
						Name:  "to2",
						Desc:  "go to 2",
						Depth: 1,
					},
				},
			},
		},
		{
			next: "to2",
			preFunc: func() {
				System[testSystem](state).toggle = true
			},
			expected: exp{
				event:  "failed to go to 2",
				prompt: nil,
				opts: []ValidOption{
					{
						Name:  "toa",
						Depth: 0,
					},
					{
						Name:  "to2",
						Desc:  "go to 2",
						Depth: 1,
					},
				},
			},
		},
		{
			next: "toa",
			expected: exp{
				event:  "",
				prompt: []string{"start"},
				opts: []ValidOption{
					{
						Name:  "tob",
						Desc:  "go to b",
						Depth: 0,
					},
					{
						Name:  "toc",
						Depth: 0,
					},
				},
			},
		},
		{
			next: "tob",
			expected: exp{
				event:  "going to b",
				prompt: nil,
				opts: []ValidOption{
					{
						Name:  "toa",
						Depth: 0,
					},
				},
			},
		},
		{
			next: "toa",
			expected: exp{
				event:  "going to c instead",
				prompt: []string{"count: 2"},
				opts: []ValidOption{
					{
						Name:  "toa",
						Depth: 0,
					},
					{
						Name:  "long3",
						Depth: 1,
					},
					{
						Name:  "to3",
						Depth: 1,
					},
					{
						Name:  "wait",
						Depth: 1,
					},
				},
			},
		},
	} {
		if step.preFunc != nil {
			step.preFunc()
		}

		event, err := world.Choose(step.next)
		assert.NoError(t, err)
		assert.Equal(t, step.expected.event, event)
		assert.Equal(t, step.expected.prompt, slices.Collect(world.Prompt()))
		assert.Equal(t, step.expected.opts, world.ValidOptions())
	}
}
