package smiwrite

import (
	"fmt"
	"io"

	"github.com/golangsnmp/gomib/mib"
)

// Option configures the emitter.
type Option func(*config)

type config struct {
	conformance  bool
	descriptions bool
	sequences    bool
}

func defaults() config {
	return config{
		conformance:  true,
		descriptions: true,
		sequences:    true,
	}
}

// WithConformance controls whether conformance constructs
// (MODULE-COMPLIANCE, OBJECT-GROUP, NOTIFICATION-GROUP, AGENT-CAPABILITIES)
// are included. Default: true.
func WithConformance(v bool) Option {
	return func(c *config) { c.conformance = v }
}

// WithDescriptions controls whether DESCRIPTION clauses are included.
// Default: true.
func WithDescriptions(v bool) Option {
	return func(c *config) { c.descriptions = v }
}

// WithSequences controls whether reconstructed SEQUENCE types are included.
// Default: true.
func WithSequences(v bool) Option {
	return func(c *config) { c.sequences = v }
}

// Write emits a single module from m as canonical SMIv2 text.
// Unresolved object OIDs, emitted types, syntax refinements, or written defaults cause an
// error before any output. Canonical output is not a lossless source format.
func Write(w io.Writer, m *mib.Mib, moduleName string, opts ...Option) error {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	mod := m.Module(moduleName)
	if mod == nil {
		return fmt.Errorf("module not found: %s", moduleName)
	}
	if err := validateModule(mod, cfg); err != nil {
		return err
	}
	e := newEmitter(w, m, cfg)
	return e.emitModule(mod)
}

// WriteAll emits multiple modules from m as canonical SMIv2 text,
// separated by blank lines. All requested modules are validated before output.
func WriteAll(w io.Writer, m *mib.Mib, moduleNames []string, opts ...Option) error {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	modules := make([]*mib.Module, len(moduleNames))
	for i, name := range moduleNames {
		mod := m.Module(name)
		if mod == nil {
			return fmt.Errorf("module not found: %s", name)
		}
		if err := validateModule(mod, cfg); err != nil {
			return err
		}
		modules[i] = mod
	}
	for i, mod := range modules {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		e := newEmitter(w, m, cfg)
		if err := e.emitModule(mod); err != nil {
			return err
		}
	}
	return nil
}
