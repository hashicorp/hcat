package hcat

// Resolver is responsible rendering Templates and invoking Commands.
// Empty but reserving the space for future use.
type Resolver struct{}

// ResolveEvent captures the whether the template dependencies have all been
// resolved and rendered in memory.
type ResolveEvent struct {
	// Complete is true if all dependencies have values and the template
	// is fully rendered (in memory).
	Complete bool

	// Contents is the rendered contents from the template.
	// Only returned when Complete is true.
	Contents []byte

	// NoChange is true if no dependencies have changes in values and therefore
	// templates were not re-rendered.
	NoChange bool
}

// Basic constructor, here for consistency and future flexibility.
func NewResolver() *Resolver {
	return &Resolver{}
}

// Watcherer is the subset of the Watcher's API that the resolver needs.
// The interface is used to make the used/required API explicit.
type Watcherer interface {
	Buffer(Notifier) bool
	Recaller(Notifier) Recaller
	Complete(Notifier) bool
}

// Templater the interface the Template provides.
// The interface is used to make the used/required API explicit.
type Templater interface {
	Notifier
	Execute(Recaller) ([]byte, error)
}

// Run the template Execute once. You should repeat calling this until
// output returns Complete as true. It uses the watcher for dependency
// lookup state. The content will be updated each pass until complete.
func (r *Resolver) Run(tmpl Templater, w Watcherer) (ResolveEvent, error) {

	// Check if this dependency has any dependencies that have been change and
	// if not, don't waste time re-rendering it.
	if w.Buffer(tmpl) {
		return ResolveEvent{Complete: false}, nil
	}

	// If Watcherer supports it, wrap the template call with the Mark-n-Sweep
	// garbage collector to stop and dereference the old/unused views.
	gcViews := func(f func() ([]byte, error)) ([]byte, error) { return f() }
	if c, ok := w.(Collector); ok {
		gcViews = func(f func() ([]byte, error)) (data []byte, err error) {
			c.Mark(tmpl)
			if data, err = f(); err == nil {
				c.Sweep(tmpl)
			}
			return data, err
		}
	}

	// Attempt to render the template, returning any missing dependencies and
	// the rendered contents. If there are any missing dependencies, the
	// contents cannot be rendered or trusted!
	output, err := gcViews(func() ([]byte, error) {
		return tmpl.Execute(w.Recaller(tmpl))
	})
	switch {
	case err == ErrNoNewValues || err == nil:
	default:
		return ResolveEvent{}, err
	}

	return ResolveEvent{
		Complete: w.Complete(tmpl),
		Contents: output,
		NoChange: err == ErrNoNewValues,
	}, nil
}
