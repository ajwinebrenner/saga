package saga

import (
	"reflect"

	"github.com/ajwinebrenner/saga/internal/dyn"
	"github.com/ajwinebrenner/saga/internal/system"
)

type Id string

// A collection of scenes with a default starting or entry scene.
// Scenes may only reference other scenes within the same group.
// Persist retains the last scene visited when traversing back to this group.
type Group struct {
	EntryScene Id
	Persist    bool
	Scenes     []Scene
}

// The main component of the world, the nodes by which one traverses events.
// A Scene could represent a location, a moment of decision, or narrative exposition.
// `Id` must be unique within the same group.
// The prompt for any given world state is comprised of `Desc` for the current scene and sub-scenes.
// `Desc` is evaluated for every prompt but is excluded if it hasn't changed for a given scene.
type Scene struct {
	Id        Id
	Desc      DynVal[string]
	Choices   []Choice // manually chosen by input
	Reroutes  []Route  // automatically followed when active
	SubScenes *Group   // more likely to be sparse (nil)
}

// The primary traversal method between scenes. The identifier `Name` must not be empty.
// Choices that appear last will take precedence when a name is used by more than one choice.
// Routes must refer to a scene in the same group, it may be the same scene the choice belongs to.
type Choice struct {
	Name      string
	Desc      DynVal[string] // Eval once when active for the current scene, describes before choosing
	Route     Route          // Standard route, `Active` applies to entire Choice
	Overrides []Route
}

// Describes the where (To) and the what (Event) of traversal between scenes.
// `Active` evaluates to false if nil, except for `Choice.Route` which is true by default.
type Route struct {
	To     Id
	Event  DynVal[string]
	Active DynVal[bool]
}

type ChoiceOpt func(*Choice)

func MakeChoice(name string, to Id, opts ...ChoiceOpt) Choice {
	o := Choice{
		Name:  name,
		Route: Route{To: to},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// Adds a description to the choice.
// The description is evaluated for current active choices.
func WithDesc(desc DynVal[string]) ChoiceOpt {
	return func(o *Choice) {
		o.Desc = desc
	}
}

// Adds an event that will occur when the choice is chosen.
// The returned string describes the choice's outcome.
func WithEvent(event DynVal[string]) ChoiceOpt {
	return func(o *Choice) {
		o.Route.Event = event
	}
}

// Modifies the choice to only be active if `condition` is true.
// Scenes will likely have at least one thread without a condition.
func WithCondition(condition DynVal[bool]) ChoiceOpt {
	return func(o *Choice) {
		o.Route.Active = condition
	}
}

// Modifies choice to use this `to` and `event` if `condition` is met.
// The first override that is applicable will be used.
func WithOverride(next Id, event DynVal[string], condition DynVal[bool]) ChoiceOpt {
	return func(o *Choice) {
		o.Overrides = append(o.Overrides, Route{
			To:     next,
			Event:  event,
			Active: condition,
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
