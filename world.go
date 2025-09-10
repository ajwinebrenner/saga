package saga

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
)

type World struct {
	root    *group
	state   *State
	prompt  prompt
	options map[string]ValidOption
}

type prompt struct {
	desc []string
	skip []bool
}

type ValidOption struct {
	Name  string
	Desc  string
	Depth uint
}

type group struct {
	current   Id
	entrance  entrance
	scenes    map[Id]scene
	subGroups map[Id]*group
}

type scene struct {
	desc    StateFunc[string]
	options []option
}

type option struct {
	name      string
	condition StateFunc[bool]
	desc      StateFunc[string]
	outcomer  outcomer
}

// Iterator over desc strings from each level of the scene tree in descending order.
// Descriptions are evaluated on update to world state, therefore this function has no side effects.
// Empty or previously seen descriptions are omitted from the returned values.
func (w *World) Prompt() iter.Seq[string] {
	return func(yield func(string) bool) {
		for i, skip := range w.prompt.skip {
			if !skip && !yield(w.prompt.desc[i]) {
				return
			}
		}
	}
}

// ValidOptions returns a list of validOption according to current world state.
// Options are sorted by depth, allowing handling of options from different levels.
// Repeated calls ValidOptions do not produce side effects from any StateFunc calls.
// If conflicting option names occur, only the deepest of the options is considered.
func (w *World) ValidOptions() []ValidOption {
	opts := slices.Collect(maps.Values(w.options))

	slices.SortFunc(opts, func(a ValidOption, b ValidOption) int {
		if depthDiff := int(a.Depth - b.Depth); depthDiff != 0 {
			return depthDiff
		}
		return cmp.Compare(a.Name, b.Name)
	})

	return opts
}

// If option is valid, the option will be used to find the next scene and associated event.
// World state is updated to reflect the outcome of the choosing this option.
// The returned string describes the outcome, but can be empty.
// If option is invalid, an InvalidOptionErr will be returned.
func (w *World) Choose(option string) (string, error) {
	desc, err := w.tryOption(option)
	if err != nil {
		return "", err
	}

	w.collect()
	return desc, nil
}

// Collects all valid options and desciptions for current world state.
// This should be called once between updates to world state as StateFuncs may have side effects.
func (w *World) collect() {
	clear(w.options)

	currentDepth := uint(0)
	currentGroup := w.root
	for currentGroup != nil {
		currentScene := currentGroup.scenes[currentGroup.current]

		for _, opt := range currentScene.options {
			if opt.condition == nil || opt.condition(w.state) {
				w.options[opt.name] = ValidOption{
					Name:  opt.name,
					Desc:  safeStateFunc(opt.desc, w.state),
					Depth: currentDepth,
				}
			}
		}

		desc := safeStateFunc(currentScene.desc, w.state)
		if len(w.prompt.desc) > int(currentDepth) {
			w.prompt.skip[currentDepth] = desc == "" || desc == w.prompt.desc[currentDepth]
			w.prompt.desc[currentDepth] = desc
		} else {
			w.prompt.skip = append(w.prompt.skip, desc == "")
			w.prompt.desc = append(w.prompt.desc, desc)
		}

		currentDepth++
		currentGroup = currentGroup.subGroups[currentGroup.current]
	}

	w.prompt.skip = w.prompt.skip[:currentDepth]
	w.prompt.desc = w.prompt.desc[:currentDepth]
}

const errCorruptWorldState = stringError("internal world state corruption")

type InvalidOptionError struct {
	name string
}

func (e InvalidOptionError) Error() string {
	return fmt.Sprintf("option %q not valid", e.name)
}

// Takes unsanitised input and applies the valid option if found.
// If valid, a description of the outcome is returned.
func (w *World) tryOption(name string) (string, error) {
	choice, ok := w.options[name]
	if !ok {
		return "", InvalidOptionError{name: name}
	}

	group := w.root
	for d := uint(0); d < choice.Depth && group != nil; d++ {
		group = group.subGroups[group.current]
	}

	// should only happen if internal state is incorrectly altered after collect
	if group == nil {
		return "", errCorruptWorldState
	}
	opts := group.scenes[group.current].options
	optIdx := slices.IndexFunc(opts, func(o option) bool { return o.name == name })
	if optIdx < 0 {
		return "", errCorruptWorldState
	}

	previousId := group.current

	outcome := opts[optIdx].outcomer.outcome(w.state)
	group.current = outcome.next

	group = group.subGroups[group.current]
	for group != nil {
		entryId := group.entrance.calc(previousId)
		if entryId == "" {
			break
		}

		group.current = entryId
		group = group.subGroups[group.current]
	}

	return outcome.desc, nil
}

type entrance struct {
	standard Id
	persist  bool
	alts     map[Id]Id
}

// `from` is the scene ID before choosing the most recent option.
// Empty string indicates no further scenes should be changed.
func (e entrance) calc(from Id) Id {
	if e.alts != nil {
		if alt, ok := e.alts[from]; ok {
			return alt
		}
	}

	if e.persist {
		return ""
	}

	return e.standard
}

type outcomer struct {
	next      Id
	event     StateFunc[string]
	overrides []Override
}

type outcome struct {
	next Id
	desc string
}

func (o outcomer) outcome(state *State) outcome {
	for _, override := range o.overrides {
		if safeStateFunc(override.Condition, state) {
			return outcome{
				next: override.Next,
				desc: safeStateFunc(override.Event, state),
			}
		}
	}

	return outcome{
		next: o.next,
		desc: safeStateFunc(o.event, state),
	}
}
