package hcat

// Resolver is responsible rendering Templates and invoking Commands.
// Empty but reserving the space for future use.
type Resolver struct{}

// ResolveEvent captures the whether the template dependencies have all been
// resolved and rendered in memory.
type ResolveEvent struct {
	// Complete is true if all dependencies have values and the template
	// is is fully rendered.
	Complete bool

	// Contents is the rendered contents from the template.
	// Only returned when Complete is true.
	Contents []byte

	// for testing, need way to test for missing dependencies case
	missing bool
}

// Basic constructor, here for consistency and future flexibility.
func NewResolver() *Resolver {
	return &Resolver{}
}

// Watcherer is the subset of the Watcher's API that the resolver needs.
// The interface is used to make the used/required API explicit.
type Watcherer interface {
	Buffer(tmplID string) bool
	Recaller(Notifier) Recaller
	Complete(Notifier) bool
}

// Templater the interface the Template provides.
// The interface is used to make the used/required API explicit.
type Templater interface {
	Execute(Watcherer) ([]byte, error)
	ID() string
}

// Run the template Execute once. You should repeat calling this until
// output returns Complete as true. It uses the watcher for dependency
// lookup state. The content will be updated each pass until complete.
func (r *Resolver) Run(tmpl Templater, w Watcherer) (ResolveEvent, error) {

	// Check if this dependency has any dependencies that have been change and
	// if not, don't waste time re-rendering it.
	if w.Buffer(tmpl.ID()) {
		return ResolveEvent{Complete: false}, nil
	}

	// Attempt to render the template, returning any missing dependencies and
	// the rendered contents. If there are any missing dependencies, the
	// contents cannot be rendered or trusted!
	output, err := tmpl.Execute(w)
	switch err {
	case nil:
	case ErrMissingValues:
		return ResolveEvent{missing: true}, nil
	case ErrNoNewValues:
		return ResolveEvent{}, nil
	default:
		return ResolveEvent{}, err
	}

	return ResolveEvent{Complete: true, Contents: output}, nil
}
