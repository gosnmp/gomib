package smiwrite

import (
	"fmt"

	"github.com/golangsnmp/gomib/mib"
)

// validateModule checks metadata that the emitter would otherwise replace or
// omit. Source spans distinguish absent defaults from unresolved written ones.
func validateModule(mod *mib.Module, cfg config) error {
	unresolved := func(name, clause string) error {
		return fmt.Errorf("cannot emit %s::%s: unresolved %s", mod.Name(), name, clause)
	}
	for _, typ := range collectDefinitions(mod, cfg).textualConvs {
		if typ.EffectiveBase() == mib.BaseUnknown {
			return unresolved(typ.Name(), "type")
		}
	}
	for _, obj := range mod.Objects() {
		if obj.Node() == nil || obj.OID() == nil {
			return unresolved(obj.Name(), "OID")
		}
		if obj.Kind() != mib.KindTable && obj.Kind() != mib.KindRow &&
			(obj.Type() == nil || obj.Type().EffectiveBase() == mib.BaseUnknown) {
			return unresolved(obj.Name(), "SYNTAX")
		}
		span := obj.DefaultValueSpan()
		if span.Start != span.End && obj.DefaultValue().IsZero() {
			return unresolved(obj.Name(), "DEFVAL")
		}
	}
	if !cfg.conformance {
		return nil
	}
	for _, compliance := range mod.Compliances() {
		for _, module := range compliance.Modules() {
			for _, obj := range module.Objects {
				if unresolvedSyntax(obj.Syntax) || unresolvedSyntax(obj.WriteSyntax) {
					return unresolved(compliance.Name()+"/"+obj.Object, "syntax refinement")
				}
			}
		}
	}
	for _, capability := range mod.Capabilities() {
		for _, module := range capability.Supports() {
			for i := range module.ObjectVariations {
				obj := &module.ObjectVariations[i]
				if unresolvedSyntax(obj.Syntax) || unresolvedSyntax(obj.WriteSyntax) {
					return unresolved(capability.Name()+"/"+obj.Object, "syntax refinement")
				}
				if obj.DefValSpan.Start != obj.DefValSpan.End && obj.DefVal.IsZero() {
					return unresolved(capability.Name()+"/"+obj.Object, "DEFVAL")
				}
			}
		}
	}
	return nil
}

func unresolvedSyntax(sc *mib.SyntaxConstraints) bool {
	return sc != nil && (sc.Type == nil || sc.Type.EffectiveBase() == mib.BaseUnknown)
}
