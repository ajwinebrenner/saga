package saga

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"reflect"
	"slices"

	"github.com/ajwinebrenner/saga/internal/errs"
	"github.com/ajwinebrenner/saga/internal/system"
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
	ErrEmptyId     = errs.Static("scene IDs must have a length greater than 0")
	ErrEmptyGroup  = errs.Static("groups must contain at least one scene")
	ErrEmptyChoice = errs.Static("choice names must have a length greater than 0")
)

// Builds a world using the root group and any systems needed by dynamic values.
// Any problems with scenes or choices that would cause an error during world traversal
// will cause an error during build detailing which areas need remediation.
func Build(root *Group, systems []any) (*World, error) {
	builder := newBuilder()

	r, err := builder.convertGroup(root)
	if err != nil {
		return nil, err
	}

	for pending := range builder.pendingGroups() {
		group, err := builder.convertGroup(pending.group)
		if err != nil {
			return nil, fmt.Errorf("building sub-scenes for %q: %w", pending.from, err)
		}

		pending.parent.groups[pending.from] = group
	}

	collection := system.NewCollection()
	for i, s := range systems {
		if err = collection.Add(s); err != nil {
			return nil, fmt.Errorf("adding system %d to collection: %w", i, err)
		}
	}

	systemErrors := make([]error, 0, len(builder.systemsNeeded))
	for sysType := range builder.systemsNeeded {
		_, err := collection.Get(sysType)
		systemErrors = append(systemErrors, err)
	}
	if err = errors.Join(systemErrors...); err != nil {
		return nil, fmt.Errorf("missing necessary systems: %w", err)
	}

	world := &World{
		root:    r,
		state:   collection,
		choices: make(map[string]ActiveChoice),
	}

	world.collect()
	return world, nil
}

type builder struct {
	dupeSceneIds    map[Id]struct{}
	unknownSceneIds map[Id]struct{}
	systemsNeeded   map[reflect.Type]struct{}
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
		systemsNeeded:   make(map[reflect.Type]struct{}),
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

	var err error
	existing := make(map[Id]struct{})
	converted := &group{
		current: g.EntryScene,
		entry:   g.EntryScene,
		persist: g.Persist,
		scenes:  make(map[Id]scene, len(g.Scenes)),
		groups:  make(map[Id]*group),
	}

	b.unknownSceneIds[g.EntryScene] = struct{}{}

	for _, s := range g.Scenes {
		if s.Id == "" {
			return nil, ErrEmptyId
		}

		delete(b.unknownSceneIds, s.Id)
		if _, exists := existing[s.Id]; exists {
			b.dupeSceneIds[s.Id] = struct{}{}
		} else {
			existing[s.Id] = struct{}{}
		}

		converted.scenes[s.Id], err = b.convertScene(existing, s)
		if err != nil {
			// TODO: can we report the full scene trail
			return nil, fmt.Errorf("building scene %q: %w", s.Id, err)
		}

		if s.SubScenes != nil {
			b.pending = append(b.pending, pendingGroup{
				parent: converted,
				group:  s.SubScenes,
				from:   s.Id,
			})
		}
	}

	if len(b.unknownSceneIds) > 0 {
		return nil, UnknownIdError{ids: b.unknownSceneIds}
	}
	if len(b.dupeSceneIds) > 0 {
		return nil, DuplicateIdError{ids: b.dupeSceneIds}
	}
	return converted, nil
}

func (b *builder) convertScene(existing map[Id]struct{}, s Scene) (scene, error) {
	err := addDynCheck(b, s.Desc)
	if err != nil {
		return scene{}, fmt.Errorf("desc: %w", err)
	}

	choices := make([]choice, 0, len(s.Choices))
	for _, ch := range s.Choices {
		if ch.Name == "" {
			return scene{}, ErrEmptyChoice
		}

		if err = addDynCheck(b, ch.Desc); err != nil {
			return scene{}, fmt.Errorf("%q: desc: %w", ch.Name, err)
		}

		if err = b.checkRoute(existing, ch.Route); err != nil {
			return scene{}, fmt.Errorf("%q: route: %w", ch.Name, err)
		}

		overrides, err := b.validRoutes(existing, ch.Overrides)
		if err != nil {
			return scene{}, fmt.Errorf("%q: override%w", ch.Name, err)
		}

		choices = append(choices, choice{
			name:      ch.Name,
			desc:      ch.Desc,
			route:     ch.Route,
			overrides: overrides,
		})
	}

	reroutes, err := b.validRoutes(existing, s.Reroutes)
	if err != nil {
		return scene{}, fmt.Errorf("reroute%w", err)
	}

	return scene{
		desc:     s.Desc,
		choices:  choices,
		reroutes: reroutes,
	}, nil
}

func (b *builder) validRoutes(existing map[Id]struct{}, routes []Route) ([]Route, error) {
	var (
		valid = make([]Route, 0, len(routes))
		err   error
	)

	for i, r := range routes {
		if err = b.checkRoute(existing, r); err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}

		if r.Active != nil {
			valid = append(valid, r)
		}
	}

	// if all have nil Active can release allocated slice capacity
	// TODO: add warning and count for these
	if len(valid) == 0 {
		return nil, nil
	}

	return valid, nil
}

func (b *builder) checkRoute(existing map[Id]struct{}, route Route) error {
	if _, exists := existing[route.To]; !exists {
		b.unknownSceneIds[route.To] = struct{}{}
	}

	err := addDynCheck(b, route.Active)
	if err != nil {
		return fmt.Errorf("active: %w", err)
	}
	err = addDynCheck(b, route.Event)
	if err != nil {
		return fmt.Errorf("event: %w", err)
	}

	return nil
}

func addDynCheck[T any](b *builder, val DynVal[T]) error {
	if val == nil {
		return nil
	}

	sysTypes, err := val.Systems()
	if err != nil {
		return err
	}

	for _, sysType := range sysTypes {
		b.systemsNeeded[sysType] = struct{}{}
	}

	return nil
}
