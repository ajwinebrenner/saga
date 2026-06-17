package saga

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"

	"github.com/ajwinebrenner/saga/internal/errs"
	"github.com/ajwinebrenner/saga/internal/system"
)

type World struct {
	root    *group
	state   *system.Collection
	prompt  prompt
	choices map[string]ActiveChoice
}

type prompt struct {
	desc []string
	same []bool // (as previous)
}

type ActiveChoice struct {
	Name  string
	Desc  string
	Depth uint
}

type group struct {
	entry   Id
	current Id
	persist bool
	scenes  map[Id]scene
	groups  map[Id]*group
}

type scene struct {
	desc     DynVal[string]
	choices  []choice
	reroutes []Route
}

type choice struct {
	name      string
	desc      DynVal[string]
	route     Route
	overrides []Route
}

// Iterator over scene desc strings from each level of the world in descending order.
// Descriptions are evaluated on update to world state, therefore this function has no side effects.
// Empty or previously seen descriptions are omitted from the returned values.
func (w *World) Prompt() iter.Seq[string] {
	return func(yield func(string) bool) {
		for i, skip := range w.prompt.same {
			if !skip && !yield(w.prompt.desc[i]) {
				return
			}
		}
	}
}

// Choices returns a list of ActiveChoice according to current world state.
// Choices are sorted by depth, allowing handling of threads from different levels.
// Repeated calls to Choices do not produce side effects from any DynVal calls.
// If conflicting choice names occur, only the deepest choice is considered.
func (w *World) Choices() []ActiveChoice {
	threads := slices.Collect(maps.Values(w.choices))

	slices.SortFunc(threads, func(a ActiveChoice, b ActiveChoice) int {
		if depthDiff := int(a.Depth - b.Depth); depthDiff != 0 {
			return depthDiff
		}
		return cmp.Compare(a.Name, b.Name)
	})

	return threads
}

// If choice is active, the choice will be used to find the next scene and associated event.
// World state is updated to reflect the outcome of the choosing this choice.
// The returned slice describes the choice outcome and any reroute events, but can be nil.
// If choice is inactive or doesn't exist, a ChoiceNotFoundError will be returned.
// Reroutes are always applied in order of descending depth
// and before evaluating the prompt and active choices.
func (w *World) Choose(name string) ([]string, error) {
	desc, err := w.followChoice(name)
	if err != nil {
		return nil, err
	}

	w.collect()
	return desc, nil
}

// Collects all active choices and scene descriptions for current world state.
// This should be called once between updates to world state as dynamic values may have side effects.
func (w *World) collect() {
	clear(w.choices)

	currentDepth := uint(0)
	currentGroup := w.root
	for currentGroup != nil {
		currentScene := currentGroup.scenes[currentGroup.current]

		for _, choices := range currentScene.choices {
			if choices.route.Active == nil || choices.route.Active.Eval(w.state) {
				w.choices[choices.name] = ActiveChoice{
					Name:  choices.name,
					Desc:  safeEval(choices.desc, w.state),
					Depth: currentDepth,
				}
			}
		}

		desc := safeEval(currentScene.desc, w.state)
		if len(w.prompt.desc) > int(currentDepth) {
			w.prompt.same[currentDepth] = desc == "" || desc == w.prompt.desc[currentDepth]
			w.prompt.desc[currentDepth] = desc
		} else {
			w.prompt.same = append(w.prompt.same, desc == "")
			w.prompt.desc = append(w.prompt.desc, desc)
		}

		currentDepth++
		currentGroup = currentGroup.groups[currentGroup.current]
	}

	w.prompt.same = w.prompt.same[:currentDepth]
	w.prompt.desc = w.prompt.desc[:currentDepth]
}

const (
	errCorruptWorldState = errs.Static("internal world state corruption")
	errMissingScene      = errs.Static("expected scene not present in build")
)

type ChoiceNotFoundError struct {
	name string
}

func (e ChoiceNotFoundError) Error() string {
	return fmt.Sprintf("choice %q not found", e.name)
}

// Takes unsanitised input and traverses the active choice if found.
// If active, a description of the outcome is returned.
func (w *World) followChoice(name string) ([]string, error) {
	chosen, ok := w.choices[name]
	if !ok {
		return nil, ChoiceNotFoundError{name: name}
	}

	grp := w.root
	for d := uint(0); d < chosen.Depth && grp != nil; d++ {
		grp = grp.groups[grp.current]
	}

	// should only happen if internal state is incorrectly altered after collect
	if grp == nil {
		return nil, errCorruptWorldState
	}
	choices := grp.scenes[grp.current].choices
	choiceIdx := slices.IndexFunc(choices, func(c choice) bool { return c.name == name })
	if choiceIdx < 0 {
		return nil, errCorruptWorldState // should be in sync with w.choices
	}

	var outcomes []string

	outcome := choices[choiceIdx].outcome(w.state)
	if outcome.desc != "" {
		outcomes = append(outcomes, outcome.desc)
	}

	err := grp.update(outcome.next)
	if err != nil {
		return nil, err
	}

	// reroutes
	grp = w.root
	scene := grp.scenes[grp.current]

	for grp != nil {
		var active *Route
		for _, rr := range scene.reroutes {
			if safeEval(rr.Active, w.state) {
				active = &rr
				break
			}
		}

		if active != nil {
			grp.update(active.To)
			if outcome := safeEval(active.Event, w.state); outcome != "" {
				outcomes = append(outcomes, outcome)
			}

			if active.To != grp.current { // stop infinite loop
				continue // keep checking for reroutes before descent
			}
		}

		grp = grp.groups[grp.current]
	}

	return outcomes, err
}

func (g *group) update(next Id) error {
	if _, ok := g.scenes[next]; !ok {
		return errCorruptWorldState
	}

	g.current = next
	sub := g.groups[next]
	for sub != nil && !sub.persist {
		sub.current = sub.entry
		sub = sub.groups[sub.entry]
	}

	return nil
}

type outcome struct {
	next Id
	desc string
}

func (c choice) outcome(state *system.Collection) outcome {
	for _, override := range c.overrides {
		if safeEval(override.Active, state) {
			return outcome{
				next: override.To,
				desc: safeEval(override.Event, state),
			}
		}
	}

	return outcome{
		next: c.route.To,
		desc: safeEval(c.route.Event, state),
	}
}

func safeEval[T any](v DynVal[T], state *system.Collection) T {
	if v == nil {
		var empty T
		return empty
	}
	return v.Eval(state)
}
