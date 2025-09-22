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
	ErrEmptySkein  = errs.Static("skeins must contain at least one scene")
	ErrEmptyThread = errs.Static("thread names must have a length greater than 0")
)

// Weave builds a world using the root skein and any systems needed by dynamic values.
// Any problems with scenes or threads that would cause an error during world traversal
// will cause an error to be returned detailing which areas need remediation.
func Weave(root *Skein, systems []any) (*World, error) {
	builder := newBuilder()

	r, err := builder.convertSkein(root)
	if err != nil {
		return nil, err
	}

	for pending := range builder.pendingSkeins() {
		skein, err := builder.convertSkein(pending.skein)
		if err != nil {
			return nil, err
		}

		pending.parent.skeins[pending.from] = skein
	}

	coll := system.NewCollection()
	for i, s := range systems {
		if err = coll.Add(s); err != nil {
			return nil, fmt.Errorf("adding system %d to collection: %w", i, err)
		}
	}

	systemErrors := make([]error, 0, len(builder.systemsNeeded))
	for sysType := range builder.systemsNeeded {
		_, err := coll.Get(sysType)
		systemErrors = append(systemErrors, err)
	}
	if err = errors.Join(systemErrors...); err != nil {
		return nil, fmt.Errorf("missing necessary systems: %w", err)
	}

	world := &World{
		root:    r,
		systems: coll,
		threads: make(map[string]ActiveThread),
	}

	world.collect()
	return world, nil
}

type builder struct {
	dupeSceneIds    map[Id]struct{}
	unknownSceneIds map[Id]struct{}
	systemsNeeded   map[reflect.Type]struct{}
	pending         []pendingSkein
}

type pendingSkein struct {
	parent *skein
	skein  *Skein
	from   Id
}

func newBuilder() *builder {
	return &builder{
		dupeSceneIds:    make(map[Id]struct{}),
		unknownSceneIds: make(map[Id]struct{}),
		systemsNeeded:   make(map[reflect.Type]struct{}),
	}
}

func (b *builder) pendingSkeins() iter.Seq[pendingSkein] {
	return func(yield func(pendingSkein) bool) {
		for len(b.pending) > 0 {
			next := b.pending[len(b.pending)-1]
			b.pending = b.pending[:len(b.pending)-1]

			if !yield(next) {
				return
			}
		}
	}
}

func (b *builder) convertSkein(g *Skein) (*skein, error) {
	if g == nil || len(g.Scenes) == 0 {
		return nil, ErrEmptySkein
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

	converted := &skein{
		current:  g.EntryScene,
		entrance: b.createEntrance(existing, g.EntryScene, g.Persist, g.AltEntries),
		scenes:   make(map[Id]scene, len(g.Scenes)),
		skeins:   make(map[Id]*skein),
	}

	var err error
	for _, s := range g.Scenes {
		converted.scenes[s.Id], err = b.convertScene(existing, s)
		if err != nil {
			return nil, fmt.Errorf("weaving scene %q: %w", s.Id, err)
		}

		if s.Skein != nil {
			b.pending = append(b.pending, pendingSkein{
				parent: converted,
				skein:  s.Skein,
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

func (b *builder) convertScene(existing map[Id]struct{}, s Scene) (scene, error) {
	err := addDynCheck(b, s.Desc)
	if err != nil {
		return scene{}, fmt.Errorf("desc: %w", err)
	}

	threads := make([]thread, 0, len(s.Threads))

	for _, o := range s.Threads {
		if o.Name == "" {
			return scene{}, ErrEmptyThread
		}

		if err = addDynCheck(b, o.Desc); err != nil {
			return scene{}, fmt.Errorf("%q: desc: %w", o.Name, err)
		}
		if err = addDynCheck(b, o.Condition); err != nil {
			return scene{}, fmt.Errorf("%q: condition: %w", o.Name, err)
		}

		outcomer, err := b.createOutcomer(existing, o.Next, o.Event, o.Overrides)
		if err != nil {
			return scene{}, fmt.Errorf("%q: %w", o.Name, err)
		}

		threads = append(threads, thread{
			name:      o.Name,
			desc:      o.Desc,
			condition: o.Condition,
			outcomer:  outcomer,
		})
	}

	return scene{
		desc:    s.Desc,
		threads: threads,
	}, err
}

func (b *builder) createOutcomer(existing map[Id]struct{}, next Id, event DynVal[string], overrides []Override) (outcomer, error) {
	if _, exists := existing[next]; !exists {
		b.unknownSceneIds[next] = struct{}{}
	}

	err := addDynCheck(b, event)
	if err != nil {
		return outcomer{}, fmt.Errorf("event: %w", err)
	}

	// assume capacity for all as happy path
	validOverrides := make([]Override, 0, len(overrides))
	for i, override := range overrides {
		if _, exists := existing[override.Next]; !exists {
			b.unknownSceneIds[override.Next] = struct{}{}
		}
		if override.Condition == nil {
			continue
		}

		if err = addDynCheck(b, override.Condition); err != nil {
			return outcomer{}, fmt.Errorf("override %d: condition: %w", i, err)
		}
		if err = addDynCheck(b, override.Event); err != nil {
			return outcomer{}, fmt.Errorf("override %d: event: %w", i, err)
		}

		validOverrides = append(validOverrides, override)
	}

	// release capacity while still allowing build
	if len(validOverrides) == 0 {
		validOverrides = nil
	}

	return outcomer{
		next:      next,
		event:     event,
		overrides: validOverrides,
	}, nil
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
