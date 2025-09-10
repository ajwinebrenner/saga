package saga

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func testGroup() Group {
	return Group{
		EntryScene: "a",
		Scenes: []Scene{
			{
				Id:   "a",
				Desc: Just("start"),
				Options: []Option{
					{
						Name:  "tob",
						Next:  "b",
						Desc:  Just("go to b"),
						Event: Just("going to b"),
					},
					{
						Name: "toc",
						Next: "c",
						Overrides: []Override{
							{
								Next:      "b",
								Condition: nil,
							},
						},
					},
				},
			},
			{
				Id: "b",
				Options: []Option{
					{
						Name: "toa",
						Next: "a",
						Overrides: []Override{
							{
								Next:  "c",
								Event: Just("going to c instead"),
								Condition: func(state *State) bool {
									return System[testSystem](state).toggle
								}},
						},
					},
				},
			},
			{
				Id: "c",
				Desc: func(state *State) string {
					return fmt.Sprintf("count: %d", System[testSystem](state).count)
				},
				Options: []Option{
					{
						Name: "toa",
						Next: "a",
						Condition: func(state *State) bool {
							return System[testSystem](state).toggle
						},
					},
				},
				Group: &Group{
					EntryScene: "1",
					Persist:    true,
					AltEntries: []AltEntry{
						{Scene: "2", From: "b"},
					},
					Scenes: []Scene{
						{
							Id:   "1",
							Desc: Just("subscene"),
							Options: []Option{
								{
									Name:      "to2",
									Next:      "2",
									Desc:      Just("go to 2"),
									Event:     Just("going to 2"),
									Condition: Just(true),
									Overrides: []Override{
										{
											Next:  "1",
											Event: Just("failed to go to 2"),
											Condition: func(state *State) bool {
												return System[testSystem](state).toggle
											},
										},
									},
								},
							},
						},
						{
							Id: "2",
							Options: []Option{
								{
									Name: "to3",
									Next: "3",
								},
								{
									Name:  "long3",
									Next:  "3",
									Event: Just("taking the long way to 3"),
								},
								{
									Name: "wait",
									Next: "2",
								},
							},
						},
						{
							Id:   "3",
							Desc: Just("you are at 3"),
							Options: []Option{
								{
									Name: "to1",
									Next: "1",
								},
							},
						},
					},
				},
			},
		},
	}
}

type testSystem struct {
	toggle bool
	count  int
}

func testState() *State {
	state := NewState()
	state.AddSystem(&testSystem{
		toggle: false,
		count:  0,
	})
	return state
}

func TestBuildPassing(t *testing.T) {
	root := testGroup()
	state := testState()

	expected := &World{
		root: &group{
			current: "a",
			entrance: entrance{
				standard: "a",
				persist:  false,
				alts:     nil,
			},
			scenes: map[Id]scene{
				"a": {
					desc: root.Scenes[0].Desc,
					options: []option{
						{
							name:      "tob",
							condition: nil,
							desc:      root.Scenes[0].Options[0].Desc,
							outcomer: outcomer{
								next:      "b",
								event:     root.Scenes[0].Options[0].Event,
								overrides: nil,
							},
						},
						{
							name:      "toc",
							condition: nil,
							desc:      nil,
							outcomer: outcomer{
								next:      "c",
								event:     nil,
								overrides: nil,
							},
						},
					},
				},
				"b": {
					desc: nil,
					options: []option{
						{
							name:      "toa",
							condition: nil,
							desc:      nil,
							outcomer: outcomer{
								next:      "a",
								event:     nil,
								overrides: root.Scenes[1].Options[0].Overrides,
							},
						},
					},
				},
				"c": {
					desc: root.Scenes[2].Desc,
					options: []option{
						{
							name:      "toa",
							condition: root.Scenes[2].Options[0].Condition,
							desc:      nil,
							outcomer: outcomer{
								next:      "a",
								event:     nil,
								overrides: nil,
							},
						},
					},
				},
			},
			subGroups: map[Id]*group{
				"c": {
					current: "1",
					entrance: entrance{
						standard: "1",
						persist:  true,
						alts: map[Id]Id{
							"b": "2",
						},
					},
					scenes: map[Id]scene{
						"1": {
							desc: root.Scenes[2].Group.Scenes[0].Desc,
							options: []option{
								{
									name:      "to2",
									condition: root.Scenes[2].Group.Scenes[0].Options[0].Condition,
									desc:      root.Scenes[2].Group.Scenes[0].Options[0].Desc,
									outcomer: outcomer{
										next:      "2",
										event:     root.Scenes[2].Group.Scenes[0].Options[0].Event,
										overrides: root.Scenes[2].Group.Scenes[0].Options[0].Overrides,
									},
								},
							},
						},
						"2": {
							desc: nil,
							options: []option{
								{
									name:      "to3",
									condition: nil,
									desc:      nil,
									outcomer: outcomer{
										next:      "3",
										event:     nil,
										overrides: nil,
									},
								},
								{
									name:      "long3",
									condition: nil,
									desc:      nil,
									outcomer: outcomer{
										next:      "3",
										event:     root.Scenes[2].Group.Scenes[1].Options[1].Event,
										overrides: nil,
									},
								},
								{
									name:      "wait",
									condition: nil,
									desc:      nil,
									outcomer: outcomer{
										next:      "2",
										event:     nil,
										overrides: nil,
									},
								},
							},
						},
						"3": {
							desc: root.Scenes[2].Group.Scenes[2].Desc,
							options: []option{
								{
									name:      "to1",
									condition: nil,
									desc:      nil,
									outcomer: outcomer{
										next:      "1",
										event:     nil,
										overrides: nil,
									},
								},
							},
						},
					},
					subGroups: map[Id]*group{},
				},
			},
		},
		state: state,
		prompt: prompt{
			desc: []string{"start"},
			skip: []bool{false},
		},
		options: map[string]ValidOption{
			"tob": {
				Name:  "tob",
				Desc:  "go to b",
				Depth: 0,
			},
			"toc": {
				Name:  "toc",
				Desc:  "",
				Depth: 0,
			},
		},
	}

	actual, err := Build(&root, state)
	assert.NoError(t, err)
	diff := cmp.Diff(expected, actual,
		cmp.Exporter(func(t reflect.Type) bool {
			return true
		}),
		// root needs to compare function addresses
		cmp.Transformer("sfbool", func(f StateFunc[bool]) string {
			return fmt.Sprint(f)
		}),
		cmp.Transformer("sfstring", func(f StateFunc[string]) string {
			return fmt.Sprint(f)
		}),
	)
	if diff != "" {
		t.Logf("builds are not equal\n%s", diff)
		t.Fail()
	}
}
