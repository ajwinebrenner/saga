package saga

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ajwinebrenner/saga/internal/system"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func testSkein() Skein {
	return Skein{
		EntryScene: "a",
		Scenes: []Scene{
			{
				Id:   "a",
				Desc: Just("start"),
				Threads: []Thread{
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
				Threads: []Thread{
					{
						Name: "toa",
						Next: "a",
						Overrides: []Override{
							{
								Next:  "c",
								Event: Just("going to c instead"),
								Condition: Dyn(func(s *testSystem) bool {
									return s.toggle
								}),
							},
						},
					},
				},
			},
			{
				Id: "c",
				Desc: Dyn(func(sys *testSystem) string {
					return fmt.Sprintf("count: %d", sys.count)
				}),
				Threads: []Thread{
					{
						Name: "toa",
						Next: "a",
						Condition: Dyn(func(sys *testSystem) bool {
							return sys.toggle
						}),
					},
				},
				Skein: &Skein{
					EntryScene: "1",
					Persist:    true,
					AltEntries: []AltEntry{
						{Scene: "2", From: "b"},
					},
					Scenes: []Scene{
						{
							Id:   "1",
							Desc: Just("sub-scene"),
							Threads: []Thread{
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
											Condition: Dyn(func(sys *testSystem) bool {
												return sys.toggle
											}),
										},
									},
								},
							},
						},
						{
							Id: "2",
							Threads: []Thread{
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
							Threads: []Thread{
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

func TestBuildPassing(t *testing.T) {
	root := testSkein()

	expectedColl := system.NewCollection()
	expectedColl.Add(&testSystem{})
	expected := &World{
		root: &skein{
			current: "a",
			entrance: entrance{
				standard: "a",
				persist:  false,
				alts:     nil,
			},
			scenes: map[Id]scene{
				"a": {
					desc: root.Scenes[0].Desc,
					threads: []thread{
						{
							name:      "tob",
							condition: nil,
							desc:      root.Scenes[0].Threads[0].Desc,
							outcomer: outcomer{
								next:      "b",
								event:     root.Scenes[0].Threads[0].Event,
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
					threads: []thread{
						{
							name:      "toa",
							condition: nil,
							desc:      nil,
							outcomer: outcomer{
								next:      "a",
								event:     nil,
								overrides: root.Scenes[1].Threads[0].Overrides,
							},
						},
					},
				},
				"c": {
					desc: root.Scenes[2].Desc,
					threads: []thread{
						{
							name:      "toa",
							condition: root.Scenes[2].Threads[0].Condition,
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
			skeins: map[Id]*skein{
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
							desc: root.Scenes[2].Skein.Scenes[0].Desc,
							threads: []thread{
								{
									name:      "to2",
									condition: root.Scenes[2].Skein.Scenes[0].Threads[0].Condition,
									desc:      root.Scenes[2].Skein.Scenes[0].Threads[0].Desc,
									outcomer: outcomer{
										next:      "2",
										event:     root.Scenes[2].Skein.Scenes[0].Threads[0].Event,
										overrides: root.Scenes[2].Skein.Scenes[0].Threads[0].Overrides,
									},
								},
							},
						},
						"2": {
							desc: nil,
							threads: []thread{
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
										event:     root.Scenes[2].Skein.Scenes[1].Threads[1].Event,
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
							desc: root.Scenes[2].Skein.Scenes[2].Desc,
							threads: []thread{
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
					skeins: map[Id]*skein{},
				},
			},
		},
		systems: expectedColl,
		prompt: prompt{
			desc: []string{"start"},
			skip: []bool{false},
		},
		threads: map[string]ActiveThread{
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

	type testBool func(*testSystem) bool
	type testString func(*testSystem) string

	actual, err := Weave(&root, []any{&testSystem{}})
	assert.NoError(t, err)
	diff := cmp.Diff(expected, actual,
		cmp.Exporter(func(t reflect.Type) bool {
			return true
		}),
		// root needs to compare function addresses
		cmp.Transformer("fbool", func(f testBool) string {
			return fmt.Sprint(f)
		}),
		cmp.Transformer("fstring", func(f testString) string {
			return fmt.Sprint(f)
		}),
	)
	if diff != "" {
		t.Logf("worlds are not equivalent\n%s", diff)
		t.Fail()
	}
}
