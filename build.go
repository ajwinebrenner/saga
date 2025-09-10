package saga

import (
	"fmt"
	"iter"
	"maps"
	"slices"
)

type DuplicateIdError struct {
	ids map[Id]struct{}
}

func (e DuplicateIdError) Error() string {
	ids := slices.Collect(maps.Keys(e.ids))
	slices.Sort(ids)
	return fmt.Sprintf("duplicate IDs: %q", ids)
}

type UnknownIdError struct {
	ids map[Id]struct{}
}

func (e UnknownIdError) Error() string {
	ids := slices.Collect(maps.Keys(e.ids))
	slices.Sort(ids)
	return fmt.Sprintf("unknown IDs: %q", ids)
}

const (
	ErrEmptyId     = stringError("scene IDs must have a length greater than 0")
	ErrEmptyGroup  = stringError("groups must contain at least one scene")
	ErrEmptyOption = stringError("option names must have a length greater than 0")
)

func Build(root *Group, state *State) (*World, error) {
	builder := newBuilder()

	r, err := builder.convertGroup(root)
	if err != nil {
		return nil, err
	}

	for pending := range builder.pendingGroups() {
		group, err := builder.convertGroup(pending.group)
		if err != nil {
			return nil, err
		}

		pending.parent.subGroups[pending.from] = group
	}

	world := &World{
		root:    r,
		state:   state,
		options: make(map[string]ValidOption),
	}

	world.collect()
	return world, nil
}

type builder struct {
	dupeSceneIds    map[Id]struct{}
	unknownSceneIds map[Id]struct{}
	pending         []pendingGroup
}

type pendingGroup struct {
	parent *group
	group  *Group
	from   Id
}

func newBuilder() *builder {
	return &builder{
		dupeSceneIds:    make(map[Id]struct{}),
		unknownSceneIds: make(map[Id]struct{}),
	}
}

func (b *builder) pendingGroups() iter.Seq[pendingGroup] {
	return func(yield func(pendingGroup) bool) {
		for len(b.pending) > 0 {
			next := b.pending[len(b.pending)-1]
			b.pending = b.pending[:len(b.pending)-1]

			if !yield(next) {
				return
			}
		}
	}
}

func (b *builder) convertGroup(g *Group) (*group, error) {
	if g == nil || len(g.Scenes) == 0 {
		return nil, ErrEmptyGroup
	}

	existing := make(map[Id]struct{})
	for _, s := range g.Scenes {
		if s.Id == "" {
			return nil, ErrEmptyId
		}

		if _, exists := existing[s.Id]; exists {
			b.dupeSceneIds[s.Id] = struct{}{}
		}

		existing[s.Id] = struct{}{}
	}
	if len(b.dupeSceneIds) > 0 {
		return nil, DuplicateIdError{ids: b.dupeSceneIds}
	}

	converted := &group{
		current:   g.EntryScene,
		entrance:  b.createEntrance(existing, g.EntryScene, g.Persist, g.AltEntries),
		scenes:    make(map[Id]scene, len(g.Scenes)),
		subGroups: make(map[Id]*group),
	}

	for _, s := range g.Scenes {
		options := make([]option, 0, len(s.Options))

		for _, o := range s.Options {
			if o.Name == "" {
				return nil, ErrEmptyOption
			}

			options = append(options, option{
				name:      o.Name,
				desc:      o.Desc,
				condition: o.Condition,
				outcomer:  b.createOutcomer(existing, o.Next, o.Event, o.Overrides),
			})
		}

		converted.scenes[s.Id] = scene{
			desc:    s.Desc,
			options: options,
		}

		if s.Group != nil {
			b.pending = append(b.pending, pendingGroup{
				parent: converted,
				group:  s.Group,
				from:   s.Id,
			})
		}
	}

	if len(b.unknownSceneIds) > 0 {
		return nil, UnknownIdError{ids: b.unknownSceneIds}
	}
	return converted, nil
}

func (b *builder) createEntrance(existing map[Id]struct{}, entryScene Id, persist bool, altEntries []AltEntry) entrance {
	if _, exists := existing[entryScene]; !exists {
		b.unknownSceneIds[entryScene] = struct{}{}
	}

	var entryMap map[Id]Id

	if len(altEntries) > 0 {
		entryMap = make(map[Id]Id, len(altEntries))
		for _, altEntry := range altEntries {
			if _, exists := existing[altEntry.Scene]; !exists {
				b.unknownSceneIds[altEntry.Scene] = struct{}{}
			}
			entryMap[altEntry.From] = altEntry.Scene
		}
	}

	return entrance{
		standard: entryScene,
		persist:  persist,
		alts:     entryMap,
	}
}

func (b *builder) createOutcomer(existing map[Id]struct{}, next Id, event StateFunc[string], overrides []Override) outcomer {
	if _, exists := existing[next]; !exists {
		b.unknownSceneIds[next] = struct{}{}
	}

	// assume capacity for all as happy path
	validOverrides := make([]Override, 0, len(overrides))
	for _, override := range overrides {
		if _, exists := existing[override.Next]; !exists {
			b.unknownSceneIds[override.Next] = struct{}{}
		}
		if override.Condition != nil {
			validOverrides = append(validOverrides, override)
		}
	}

	// release capacity while still allowing build
	if len(validOverrides) == 0 {
		validOverrides = nil
	}

	return outcomer{
		next:      next,
		event:     event,
		overrides: validOverrides,
	}
}

func safeStateFunc[T any](f StateFunc[T], state *State) T {
	if f == nil {
		var empty T
		return empty
	}
	return f(state)
}
