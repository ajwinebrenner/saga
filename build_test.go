package saga

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/ajwinebrenner/saga/internal/system"
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
				Choices: []Choice{
					{
						Name: "tob",
						Desc: Just("go to b"),
						Route: Route{
							To:    "b",
							Event: Just("going to b"),
						},
					},
					{
						Name: "toc",
						Route: Route{
							To: "c",
						},
						Overrides: []Route{
							{
								To:     "b",
								Active: nil,
							},
						},
					},
				},
			},
			{
				Id: "b",
				Choices: []Choice{
					{
						Name: "toa",
						Route: Route{
							To: "a",
						},
						Overrides: []Route{
							{
								To:    "c",
								Event: Just("going to c instead"),
								Active: Dyn(func(s *testSystem) bool {
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
				Choices: []Choice{
					{
						Name: "toa",
						Route: Route{
							To: "a",
							Active: Dyn(func(sys *testSystem) bool {
								return sys.toggle
							}),
						},
					},
				},
				Reroutes: []Route{
					{
						To:    "a",
						Event: Just("rerouting!"),
						Active: Dyn(func(sys *testSystem) bool {
							defer func() { sys.reroute = false }()
							return sys.reroute
						}),
					},
					{
						To:    "b",
						Event: Just("forbidden reroute"),
						// should never activate as evaluated in order of appearance
						Active: Dyn(func(sys *testSystem) bool {
							return sys.reroute
						}),
					},
				},
				SubScenes: &Group{
					EntryScene: "1",
					Persist:    true,
					Scenes: []Scene{
						{
							Id:   "1",
							Desc: Just("sub-scene"),
							Choices: []Choice{
								{
									Name: "to2",
									Desc: Just("go to 2"),
									Route: Route{
										To:     "2",
										Event:  Just("going to 2"),
										Active: Just(true),
									},
									Overrides: []Route{
										{
											To:    "1",
											Event: Just("failed to go to 2"),
											Active: Dyn(func(sys *testSystem) bool {
												return sys.toggle
											}),
										},
									},
								},
								{
									Name: "secret",
									Desc: Just("a hidden way to 2"),
									Route: Route{
										To: "2",
										Active: Dyn(func(sys *testSystem) bool {
											return sys.toggle
										}),
									},
								},
							},
						},
						{
							Id: "2",
							Choices: []Choice{
								{
									Name: "to3",
									Route: Route{
										To: "3",
									},
								},
								{
									Name: "long3",
									Route: Route{
										To:    "3",
										Event: Just("taking the long way to 3"),
									},
								},
								{
									Name: "wait",
									Route: Route{
										To: "2",
									},
								},
							},
						},
						{
							Id:   "3",
							Desc: Just("you are at 3"),
							Choices: []Choice{
								{
									Name: "to1",
									Route: Route{
										To: "1",
									},
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
	reroute bool
	toggle  bool
	count   int
}

func TestBuildPassing(t *testing.T) {
	root := testGroup()

	expectedColl := system.NewCollection()
	expectedColl.Add(&testSystem{})
	expected := &World{
		root: &group{
			current: "a",
			entry:   "a",
			persist: false,
			scenes: map[Id]scene{
				"a": {
					desc: root.Scenes[0].Desc,
					choices: []choice{
						{
							name: "tob",
							desc: root.Scenes[0].Choices[0].Desc,
							route: Route{
								To:     "b",
								Event:  root.Scenes[0].Choices[0].Route.Event,
								Active: nil,
							},
							overrides: nil,
						},
						{
							name: "toc",
							desc: nil,
							route: Route{
								To:     "c",
								Event:  nil,
								Active: nil,
							},
							overrides: nil,
						},
					},
				},
				"b": {
					desc: nil,
					choices: []choice{
						{
							name: "toa",
							desc: nil,
							route: Route{
								To:     "a",
								Event:  nil,
								Active: nil,
							},
							overrides: root.Scenes[1].Choices[0].Overrides,
						},
					},
				},
				"c": {
					desc: root.Scenes[2].Desc,
					choices: []choice{
						{
							name: "toa",
							desc: nil,
							route: Route{
								To:     "a",
								Event:  nil,
								Active: root.Scenes[2].Choices[0].Route.Active,
							},
							overrides: nil,
						},
					},
					reroutes: root.Scenes[2].Reroutes,
				},
			},
			groups: map[Id]*group{
				"c": {
					current: "1",
					entry:   "1",
					persist: true,
					scenes: map[Id]scene{
						"1": {
							desc: root.Scenes[2].SubScenes.Scenes[0].Desc,
							choices: []choice{
								{
									name: "to2",
									desc: root.Scenes[2].SubScenes.Scenes[0].Choices[0].Desc,
									route: Route{
										To:     "2",
										Event:  root.Scenes[2].SubScenes.Scenes[0].Choices[0].Route.Event,
										Active: root.Scenes[2].SubScenes.Scenes[0].Choices[0].Route.Active,
									},
									overrides: root.Scenes[2].SubScenes.Scenes[0].Choices[0].Overrides,
								},
								{
									name: "secret",
									desc: root.Scenes[2].SubScenes.Scenes[0].Choices[1].Desc,
									route: Route{
										To:     "2",
										Event:  root.Scenes[2].SubScenes.Scenes[0].Choices[1].Route.Event,
										Active: root.Scenes[2].SubScenes.Scenes[0].Choices[1].Route.Active,
									},
								},
							},
						},
						"2": {
							desc: nil,
							choices: []choice{
								{
									name: "to3",

									desc: nil,
									route: Route{
										To:     "3",
										Event:  nil,
										Active: nil,
									},
									overrides: nil,
								},
								{
									name: "long3",

									desc: nil,
									route: Route{
										To:     "3",
										Event:  root.Scenes[2].SubScenes.Scenes[1].Choices[1].Route.Event,
										Active: nil,
									},
									overrides: nil,
								},
								{
									name: "wait",

									desc: nil,
									route: Route{
										To:     "2",
										Event:  nil,
										Active: nil,
									},
									overrides: nil,
								},
							},
						},
						"3": {
							desc: root.Scenes[2].SubScenes.Scenes[2].Desc,
							choices: []choice{
								{
									name: "to1",

									desc: nil,
									route: Route{
										To:     "1",
										Event:  nil,
										Active: nil,
									},
									overrides: nil,
								},
							},
						},
					},
					groups: map[Id]*group{},
				},
			},
		},
		state: expectedColl,
		prompt: prompt{
			desc: []string{"start"},
			same: []bool{false},
		},
		choices: map[string]ActiveChoice{
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

	actual, err := Build(&root, []any{&testSystem{}})
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

func BenchmarkBuild(b *testing.B) {
	const largeCount = 1_000_000

	type stubSys struct {
		b bool
	}

	scenes := make([]Scene, 0, largeCount)
	for i := range largeCount {
		scenes = append(scenes, Scene{
			Id:   Id(strconv.Itoa(i)),
			Desc: Just(fmt.Sprintf("desc %d", i)),
			Choices: []Choice{
				{
					Name: "next",
					Desc: Just("go to next"),
					Route: Route{
						To: Id(strconv.Itoa(i + 1)),
						Event: Dyn(func(s *stubSys) string {
							return "going to next"
						}),
					},
					Overrides: []Route{
						{
							To: Id(strconv.Itoa(i)),
							Event: Dyn(func(s *stubSys) string {
								return "stuck"
							}),
							Active: Dyn(func(s *stubSys) bool {
								return s.b
							}),
						},
					},
				},
			},
			Reroutes: []Route{
				{
					To: Id(strconv.Itoa(i)),
					Event: Dyn(func(s *stubSys) string {
						return "deja vu"
					}),
					Active: Dyn(func(s *stubSys) bool {
						return s.b
					}),
				},
			},
		})
	}

	scenes = append(scenes, Scene{
		Id: Id(strconv.Itoa(largeCount)),
	})

	g := Group{
		EntryScene: "1",
		Scenes:     scenes,
	}

	for b.Loop() {
		_, err := Build(&g, []any{&stubSys{}})
		if err != nil {
			b.Error(err)
		}
	}
}
