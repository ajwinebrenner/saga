package saga

type Id string

type StateFunc[T any] func(state *State) T

type Group struct {
	EntryScene Id
	Persist    bool
	AltEntries []AltEntry
	Scenes     []Scene
}

// Specifies an alternative initial scene when entering the group.
// Only applies when `From` matches the previous scene (parent sibling).
type AltEntry struct {
	Scene Id
	From  Id
}

type Scene struct {
	Id      Id
	Desc    StateFunc[string]
	Options []Option
	Group   *Group // likely to be sparse (nil)
	// Action  func(*State, string) // struct to combine with hints etc.
	// Redirects []
}

type Action struct {
	f func(*State, string) ActionResult
}

type ActionResult struct {
	Desc  string
	Valid bool
}

type Option struct {
	Name      string
	Next      Id
	Desc      StateFunc[string] // Run once when valid for the current scene, describes choice before choosing
	Event     StateFunc[string] // Run once when the option is chosen, returns description of outcome
	Condition StateFunc[bool]   // Determines whether option is valid, if nil, option is always valid
	Overrides []Override
}

// The first Override where Condition is true will replace Next and Event.
type Override struct {
	Next      Id
	Event     StateFunc[string]
	Condition StateFunc[bool]
}

func MakeScene(id Id, desc StateFunc[string], options []Option, opts ...func(*Scene)) Scene {
	n := Scene{
		Id:      id,
		Desc:    desc,
		Options: options,
	}

	for _, opt := range opts {
		opt(&n)
	}

	return n
}

// Will expand the current scene to contain subscenes.
// All subScenes will have access to parent options.
func WithSubScenes(initial Id, retain bool, scenes ...Scene) func(*Scene) {
	return func(n *Scene) {
		n.Group = &Group{
			EntryScene: initial,
			Scenes:     scenes,
			Persist:    retain,
		}
	}
}

func MakeOption(name string, next Id, desc StateFunc[string], opts ...func(*Option)) Option {
	o := Option{
		Name: name,
		Next: next,
		Desc: desc,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// Adds an event that will occur when the option is chosen.
// The returned string describes the outcome.
func WithEvent(event StateFunc[string]) func(*Option) {
	return func(o *Option) {
		o.Event = event
	}
}

// Modifies the Option to only be available if `condition` is met.
// Scenes must have at least one Option without a condition.
func WithCondition(condition StateFunc[bool]) func(*Option) {
	return func(o *Option) {
		o.Condition = condition
	}
}

// Modifies Option to use `override` outcome if `condition` is met.
// If multiple overrides are applicable, the first instance will be used.
func WithOverride(next Id, event StateFunc[string], condition StateFunc[bool]) func(*Option) {
	return func(o *Option) {
		o.Overrides = append(o.Overrides, Override{
			Next:      next,
			Event:     event,
			Condition: condition,
		})
	}
}

// A convenience function to return the constant val regardless of State.
func Just[T any](val T) StateFunc[T] {
	return func(_ *State) T {
		return val
	}
}
