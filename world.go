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
	root    *skein
	systems *system.Collection
	prompt  prompt
	threads map[string]ActiveThread
}

type prompt struct {
	desc []string
	skip []bool
}

type ActiveThread struct {
	Name  string
	Desc  string
	Depth uint
}

type skein struct {
	current  Id
	entrance entrance
	scenes   map[Id]scene
	skeins   map[Id]*skein
}

type scene struct {
	desc    DynVal[string]
	threads []thread
}

type thread struct {
	name      string
	condition DynVal[bool]
	desc      DynVal[string]
	outcomer  outcomer
}

// Iterator over scene desc strings from each level of the world in descending order.
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

// Threads returns a list of ActiveThread according to current world state.
// Threads are sorted by depth, allowing handling of threads from different levels.
// Repeated calls Threads do not produce side effects from any DynVal calls.
// If conflicting thread names occur, only the deepest thread is considered.
func (w *World) Threads() []ActiveThread {
	threads := slices.Collect(maps.Values(w.threads))

	slices.SortFunc(threads, func(a ActiveThread, b ActiveThread) int {
		if depthDiff := int(a.Depth - b.Depth); depthDiff != 0 {
			return depthDiff
		}
		return cmp.Compare(a.Name, b.Name)
	})

	return threads
}

// If thread is active, the thread will be used to find the next scene and associated event.
// World state is updated to reflect the outcome of the choosing this thread.
// The returned string describes the outcome, but can be empty.
// If thread is inactive, a ThreadNotFoundError will be returned.
func (w *World) Choose(thread string) (string, error) {
	desc, err := w.followThread(thread)
	if err != nil {
		return "", err
	}

	w.collect()
	return desc, nil
}

// Collects all active threads and descriptions for current world state.
// This should be called once between updates to world state as dynamic values may have side effects.
func (w *World) collect() {
	clear(w.threads)

	currentDepth := uint(0)
	currentSkein := w.root
	for currentSkein != nil {
		currentScene := currentSkein.scenes[currentSkein.current]

		for _, thread := range currentScene.threads {
			if thread.condition == nil || thread.condition.Eval(w.systems) {
				w.threads[thread.name] = ActiveThread{
					Name:  thread.name,
					Desc:  safeEval(thread.desc, w.systems),
					Depth: currentDepth,
				}
			}
		}

		desc := safeEval(currentScene.desc, w.systems)
		if len(w.prompt.desc) > int(currentDepth) {
			w.prompt.skip[currentDepth] = desc == "" || desc == w.prompt.desc[currentDepth]
			w.prompt.desc[currentDepth] = desc
		} else {
			w.prompt.skip = append(w.prompt.skip, desc == "")
			w.prompt.desc = append(w.prompt.desc, desc)
		}

		currentDepth++
		currentSkein = currentSkein.skeins[currentSkein.current]
	}

	w.prompt.skip = w.prompt.skip[:currentDepth]
	w.prompt.desc = w.prompt.desc[:currentDepth]
}

const errCorruptWorldState = errs.Static("internal world state corruption")

type ThreadNotFoundError struct {
	name string
}

func (e ThreadNotFoundError) Error() string {
	return fmt.Sprintf("thread %q not found", e.name)
}

// Takes unsanitised input and traverses the active thread if found.
// If active, a description of the outcome is returned.
func (w *World) followThread(name string) (string, error) {
	choice, ok := w.threads[name]
	if !ok {
		return "", ThreadNotFoundError{name: name}
	}

	skein := w.root
	for d := uint(0); d < choice.Depth && skein != nil; d++ {
		skein = skein.skeins[skein.current]
	}

	// should only happen if internal state is incorrectly altered after collect
	if skein == nil {
		return "", errCorruptWorldState
	}
	threads := skein.scenes[skein.current].threads
	threadIdx := slices.IndexFunc(threads, func(t thread) bool { return t.name == name })
	if threadIdx < 0 {
		return "", errCorruptWorldState
	}

	previousId := skein.current

	outcome := threads[threadIdx].outcomer.outcome(w.systems)
	skein.current = outcome.next

	skein = skein.skeins[skein.current]
	for skein != nil {
		entryId := skein.entrance.calc(previousId)
		if entryId == "" {
			break
		}

		skein.current = entryId
		skein = skein.skeins[skein.current]
	}

	return outcome.desc, nil
}

type entrance struct {
	standard Id
	persist  bool
	alts     map[Id]Id
}

// `from` is the scene ID before choosing the most recent thread.
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
	event     DynVal[string]
	overrides []Override
}

type outcome struct {
	next Id
	desc string
}

func (o outcomer) outcome(state *system.Collection) outcome {
	for _, override := range o.overrides {
		if safeEval(override.Condition, state) {
			return outcome{
				next: override.Next,
				desc: safeEval(override.Event, state),
			}
		}
	}

	return outcome{
		next: o.next,
		desc: safeEval(o.event, state),
	}
}

func safeEval[T any](v DynVal[T], coll *system.Collection) T {
	if v == nil {
		var empty T
		return empty
	}
	return v.Eval(coll)
}
