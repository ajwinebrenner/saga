package saga

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorld(t *testing.T) {
	root := testGroup()
	sys := testSystem{
		toggle: false,
		count:  0,
	}

	world, err := Build(&root, []any{&sys})
	require.NoError(t, err)

	type exp struct {
		outcome []string
		prompt  []string
		threads []ActiveChoice
	}

	for _, step := range []struct {
		next     string
		preFunc  func()
		expected exp
	}{
		{
			next: "tob",
			expected: exp{
				outcome: []string{"going to b"},
				prompt:  nil,
				threads: []ActiveChoice{{
					Name:  "toa",
					Desc:  "",
					Depth: 0,
				}},
			},
		},
		{
			next: "toa",
			expected: exp{
				outcome: nil,
				prompt:  []string{"start"},
				threads: []ActiveChoice{
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
				outcome: nil,
				prompt:  []string{"count: 0", "sub-scene"},
				threads: []ActiveChoice{
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
				outcome: []string{"going to 2"},
				prompt:  nil,
				threads: []ActiveChoice{
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
				sys.count++
			},
			expected: exp{
				outcome: nil,
				prompt:  []string{"count: 1"},
				threads: []ActiveChoice{
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
				sys.count++
			},
			expected: exp{
				outcome: []string{"taking the long way to 3"},
				prompt:  []string{"count: 2", "you are at 3"},
				threads: []ActiveChoice{
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
				outcome: nil,
				prompt:  []string{"sub-scene"},
				threads: []ActiveChoice{
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
				sys.toggle = true
			},
			expected: exp{
				outcome: []string{"failed to go to 2"},
				prompt:  nil,
				threads: []ActiveChoice{
					{
						Name:  "toa",
						Depth: 0,
					},
					{
						Name:  "secret",
						Desc:  "a hidden way to 2",
						Depth: 1,
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
				outcome: nil,
				prompt:  []string{"start"},
				threads: []ActiveChoice{
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
				outcome: []string{"going to b"},
				prompt:  nil,
				threads: []ActiveChoice{
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
				outcome: []string{"going to c instead"},
				prompt:  []string{"count: 2", "sub-scene"},
				threads: []ActiveChoice{
					{
						Name:  "toa",
						Depth: 0,
					},
					{
						Name:  "secret",
						Desc:  "a hidden way to 2",
						Depth: 1,
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
			next: "secret",
			preFunc: func() {
				sys.reroute = true
			},
			expected: exp{
				outcome: []string{"rerouting!"},
				prompt:  []string{"start"},
				threads: []ActiveChoice{
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
			next: "toc", // straight to 2 (persist and previous destination before reroute)
			expected: exp{
				outcome: nil,
				prompt:  []string{"count: 2"},
				threads: []ActiveChoice{
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

		outcome, err := world.Choose(step.next)
		assert.NoError(t, err)
		assert.Equal(t, step.expected.outcome, outcome)
		assert.Equal(t, step.expected.prompt, slices.Collect(world.Prompt()))
		assert.Equal(t, step.expected.threads, world.Choices())
	}
}
