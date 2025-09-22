package saga

import (
	"reflect"

	"github.com/ajwinebrenner/saga/internal/dyn"
	"github.com/ajwinebrenner/saga/internal/system"
)

type Id string

// A collection of scenes with a default starting or entry scene.
// Scenes may only reference other scenes within the same skein.
// Persist retains the last scene visited when traversing back to this skein.
type Skein struct {
	EntryScene Id
	Persist    bool
	AltEntries []AltEntry
	Scenes     []Scene
}

// Specifies an alternative initial scene when entering the skein.
// Only applies when `From` matches the previous scene (parent sibling).
type AltEntry struct {
	Scene Id
	From  Id
}

// The main component of the world. A Scene can represent a location, or moment of decision.
// `Id` must be unique within the same skein. The prompt for any given world state is comprised of
// `Desc` for each current scene, avoiding repetition when the description doesn't change.
type Scene struct {
	Id      Id
	Desc    DynVal[string]
	Threads []Thread
	Skein   *Skein // likely to be sparse (nil)
}

// A `Thread` primarily enables traversal from one scene to another. `Name` must not be an empty string.
// Threads that appear last will take precedence when a name is used by more than one thread.
// `Next` must refer to a scene in the same skein and can refer to the same scene the thread belongs to.
type Thread struct {
	Name      string
	Next      Id
	Desc      DynVal[string] // Eval once when active for the current scene, describes thread before choosing
	Event     DynVal[string] // Eval once when the thread is chosen, returns description of outcome
	Condition DynVal[bool]   // Determines whether thread is active, if nil, thread is always active
	Overrides []Override
}

// The first Override where `Condition` is true will replace `Next` and `Event` on the parent thread.
type Override struct {
	Next      Id
	Event     DynVal[string]
	Condition DynVal[bool]
}

type ThreadOption func(*Thread)

func MakeThread(name string, next Id, opts ...ThreadOption) Thread {
	o := Thread{
		Name: name,
		Next: next,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// Adds a description to the thread.
// The description is evaluated for current active threads.
func WithDesc(desc DynVal[string]) ThreadOption {
	return func(o *Thread) {
		o.Desc = desc
	}
}

// Adds an event that will occur when the thread is traversed.
// The returned string describes the outcome.
func WithEvent(event DynVal[string]) ThreadOption {
	return func(o *Thread) {
		o.Event = event
	}
}

// Modifies the thread to only be active if `condition` is true.
// Scenes will likely have at least one thread without a condition.
func WithCondition(condition DynVal[bool]) ThreadOption {
	return func(o *Thread) {
		o.Condition = condition
	}
}

// Modifies thread to use this `next` and `event` if `condition` is met.
// If multiple overrides are applicable, the first instance will be used.
func WithOverride(next Id, event DynVal[string], condition DynVal[bool]) ThreadOption {
	return func(o *Thread) {
		o.Overrides = append(o.Overrides, Override{
			Next:      next,
			Event:     event,
			Condition: condition,
		})
	}
}

// Always returns val regardless of any state.
func Just[V any](val V) DynVal[V] {
	return dyn.Static(val)
}

// Evaluates V using `fn` where the argument is a pointer to a system or an anonymous struct of multiple pointers to systems.
func Dyn[I, V any](fn func(I) V) DynVal[V] {
	if dyn.MultipleArg[I]() {
		return dyn.Multiple(fn)
	}

	return dyn.Single(fn)
}

type DynVal[T any] interface {
	Eval(*system.Collection) T
	Systems() ([]reflect.Type, error)
}
