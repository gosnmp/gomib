package gomib_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/golangsnmp/gomib/mib"
)

func loadIndexDecodeMIB(t testing.TB) *mib.Mib {
	t.Helper()
	return loadProblemMIB(t, "PROBLEM-INDEX-DECODE-MIB")
}

func compileSchema(t *testing.T, name string) *mib.IndexSchema {
	t.Helper()
	m := loadIndexDecodeMIB(t)
	object := requireObject(t, m, name)
	schema, err := mib.CompileIndexSchema(object)
	if err != nil {
		t.Fatalf("CompileIndexSchema(%s): %v", name, err)
	}
	return schema
}

func TestIndexSchemaIntegerRoundTripAndOwnership(t *testing.T) {
	m := loadIndexDecodeMIB(t)
	column := requireObject(t, m, "simpleValue")
	schema, err := mib.CompileIndexSchema(column)
	if err != nil {
		t.Fatalf("CompileIndexSchema: %v", err)
	}
	m = nil
	column = nil
	runtime.GC()

	if schema.Len() != 1 || schema.Component(0).Name() != "simpleIndex" {
		t.Fatalf("unexpected schema: len=%d name=%q", schema.Len(), schema.Component(0).Name())
	}
	wire := schema.Component(0).WireType()
	if wire.Kind() != mib.IndexWireInteger || wire.IntegerKind() != mib.IntegerIndexInteger32 {
		t.Fatalf("unexpected integer wire type: %#v", wire)
	}

	encoded, err := schema.EncodeCanonical(
		[]mib.IndexValue{mib.NewInteger32IndexValue(42)},
		mib.EncodeOptions{MaxSuffixArcs: 8},
	)
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	if !encoded.Suffix().OID().Equal(mib.OID{42}) {
		t.Fatalf("encoded suffix = %v", encoded.Suffix().OID())
	}
	decoded, err := schema.DecodeExact(encoded.Suffix().OID(), mib.DecodeOptions{MaxSuffixArcs: 8})
	if err != nil {
		t.Fatalf("DecodeExact: %v", err)
	}
	value, ok := decoded.Values()[0].Signed()
	if !ok || value != 42 {
		t.Fatalf("decoded Integer32 = %d, %v", value, ok)
	}
}

func TestIndexSchemaFramingAndComposite(t *testing.T) {
	tests := []struct {
		name      string
		framing   mib.IndexFraming
		fixed     int
		suffix    mib.OID
		valueKind mib.IndexValueKind
	}{
		{"fixedEntry", mib.IndexFramingFixed, 6, mib.OID{1, 2, 3, 4, 5, 6}, mib.IndexValueOctetString},
		{"varEntry", mib.IndexFramingLengthPrefixed, 0, mib.OID{3, 'e', 't', 'h'}, mib.IndexValueOctetString},
		{"impliedEntry", mib.IndexFramingImplied, 0, mib.OID{'e', 't', 'h'}, mib.IndexValueOctetString},
		{"oidEntry", mib.IndexFramingLengthPrefixed, 0, mib.OID{4, 1, 3, 6, 1}, mib.IndexValueObjectIdentifier},
		{"impliedOidEntry", mib.IndexFramingImplied, 0, mib.OID{1, 3, 6, 1}, mib.IndexValueObjectIdentifier},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := compileSchema(t, test.name)
			wire := schema.Component(0).WireType()
			if wire.Framing() != test.framing {
				t.Fatalf("framing = %v, want %v", wire.Framing(), test.framing)
			}
			if test.fixed > 0 {
				width, ok := wire.FixedWidth()
				if !ok || width != test.fixed {
					t.Fatalf("fixed width = %d, %v", width, ok)
				}
			}
			decoded, err := schema.DecodeExact(test.suffix, mib.DecodeOptions{MaxSuffixArcs: 16})
			if err != nil {
				t.Fatalf("DecodeExact: %v", err)
			}
			if decoded.Values()[0].Kind() != test.valueKind {
				t.Fatalf("value kind = %v, want %v", decoded.Values()[0].Kind(), test.valueKind)
			}
			encoded, err := schema.EncodeCanonical(decoded.Values(), mib.EncodeOptions{MaxSuffixArcs: 16})
			if err != nil {
				t.Fatalf("EncodeCanonical: %v", err)
			}
			if !encoded.Suffix().OID().Equal(test.suffix) {
				t.Fatalf("round trip suffix = %v, want %v", encoded.Suffix().OID(), test.suffix)
			}
		})
	}

	schema := compileSchema(t, "multiEntry")
	decoded, err := schema.DecodeExact(mib.OID{3, 192, 168, 1, 1}, mib.DecodeOptions{MaxSuffixArcs: 5})
	if err != nil || len(decoded.Components()) != 2 {
		t.Fatalf("composite decode = %#v, %v", decoded, err)
	}
}

func TestIndexSchemaExactErrorsAndConstraints(t *testing.T) {
	schema := compileSchema(t, "simpleEntry")
	_, err := schema.DecodeExact(mib.OID{42, 99}, mib.DecodeOptions{MaxSuffixArcs: 2})
	var decodeErr *mib.IndexDecodeError
	if !errors.As(err, &decodeErr) || decodeErr.Reason != mib.IndexDecodeTrailingArcs {
		t.Fatalf("trailing decode error = %#v", err)
	}

	_, err = schema.DecodeExact(mib.OID{0}, mib.DecodeOptions{MaxSuffixArcs: 1})
	if !errors.As(err, &decodeErr) || decodeErr.Reason != mib.IndexDecodeConstraintViolation {
		t.Fatalf("constraint decode error = %#v", err)
	}
	reported, err := schema.DecodeExact(mib.OID{0}, mib.DecodeOptions{MaxSuffixArcs: 1, ConstraintMode: mib.ConstraintReport})
	if err != nil || len(reported.Violations()) != 1 {
		t.Fatalf("reported constraint = %#v, %v", reported, err)
	}

	_, err = schema.EncodeCanonical([]mib.IndexValue{mib.NewInteger32IndexValue(-1)}, mib.EncodeOptions{MaxSuffixArcs: 1})
	var encodeErr *mib.IndexEncodeError
	if !errors.As(err, &encodeErr) || encodeErr.Reason != mib.IndexEncodePrimitiveDomain {
		t.Fatalf("negative encode error = %#v", err)
	}
	_, err = schema.EncodeCanonical([]mib.IndexValue{mib.NewInteger32IndexValue(42)}, mib.EncodeOptions{MaxSuffixArcs: 0})
	if !errors.As(err, &encodeErr) || encodeErr.Reason != mib.IndexEncodeSuffixTooLong {
		t.Fatalf("bounded encode error = %#v", err)
	}
}

func TestIndexSchemaConstraintNormalization(t *testing.T) {
	// DisplayString contributes a complete SIZE (0..255) constraint.
	schema := compileSchema(t, "varEntry")
	constraint := schema.Component(0).WireType().Constraint()
	ranges := constraint.Ranges()
	if constraint.State() != mib.ConstraintKnown || len(ranges) != 1 || ranges[0] != (mib.IndexRange{Min: 0, Max: 255}) {
		t.Fatalf("normalized constraint = state %v, ranges %#v", constraint.State(), ranges)
	}

	// Returned metadata and encoded suffixes do not alias schema storage.
	components := schema.Components()
	oid := components[0].ObjectOID()
	if len(oid) > 0 {
		oid[0] = 999
		if schema.Component(0).ObjectOID()[0] == 999 {
			t.Fatal("ObjectOID accessor aliases schema storage")
		}
	}
}

func TestIndexSchemaAugmentsBareAndUnresolvedSources(t *testing.T) {
	semanticsMIB := loadProblemMIB(t, "PROBLEM-SEMANTICS-MIB")
	base, err := mib.CompileIndexSchema(requireObject(t, semanticsMIB, "problemSemEntry"))
	if err != nil {
		t.Fatalf("compile base row: %v", err)
	}
	augmented, err := mib.CompileIndexSchema(requireObject(t, semanticsMIB, "problemAugExtra"))
	if err != nil {
		t.Fatalf("compile augmented column: %v", err)
	}
	if base.Len() != augmented.Len() || base.Component(0).Name() != augmented.Component(0).Name() {
		t.Fatalf("AUGMENTS schemas differ: base=%#v augmented=%#v", base.Components(), augmented.Components())
	}

	bare, err := mib.CompileIndexSchema(requireObject(t, semanticsMIB, "problemBareEntry"))
	if err != nil {
		t.Fatalf("compile bare-type index: %v", err)
	}
	if bare.Component(0).Name() != "INTEGER" || bare.Component(0).WireType().IntegerKind() != mib.IntegerIndexInteger32 {
		t.Fatalf("unexpected bare-type schema: %#v", bare.Components())
	}

	diagnosticsMIB := loadProblemMIB(t, "PROBLEM-DIAGNOSTICS-MIB")
	_, err = mib.CompileIndexSchema(requireObject(t, diagnosticsMIB, "problemDiagEntry"))
	var schemaErr *mib.IndexSchemaError
	if !errors.As(err, &schemaErr) || schemaErr.Reason != mib.IndexSchemaUnresolvedType || schemaErr.Position != 1 {
		t.Fatalf("unresolved composite INDEX error = %#v", err)
	}
}

func TestIndexSchemaRetainsIndexObjectWithUnresolvedOID(t *testing.T) {
	m := loadIndexDecodeMIB(t)
	indexObject := requireObject(t, m, "brokenOidIndex")
	if indexObject.Node() != nil || indexObject.OID() != nil {
		t.Fatalf("brokenOidIndex identity unexpectedly resolved: node=%v OID=%v", indexObject.Node(), indexObject.OID())
	}
	if indexObject.Type() == nil || indexObject.Type().EffectiveBase() != mib.BaseInteger32 {
		t.Fatalf("brokenOidIndex type = %#v", indexObject.Type())
	}
	for _, unresolved := range m.Unresolved() {
		if unresolved.Kind == mib.UnresolvedIndex && unresolved.Symbol == "brokenOidIndex" {
			t.Fatalf("known detached object reported as unresolved INDEX: %#v", unresolved)
		}
	}

	schema, err := mib.CompileIndexSchema(requireObject(t, m, "brokenOidEntry"))
	if err != nil {
		t.Fatalf("CompileIndexSchema(brokenOidEntry): %v", err)
	}
	component := schema.Component(0)
	if component.Name() != "brokenOidIndex" || component.ObjectOID() != nil {
		t.Fatalf("unexpected detached component identity: name=%q OID=%v", component.Name(), component.ObjectOID())
	}
	foundIssue := false
	for _, issue := range component.Issues() {
		if issue.Code == mib.IndexSchemaUnresolvedObjectIdentity {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Fatalf("component issues = %#v, want unresolved object identity", component.Issues())
	}
	if _, err := schema.EncodeCanonical(
		[]mib.IndexValue{mib.NewInteger32IndexValue(7)},
		mib.EncodeOptions{MaxSuffixArcs: 1},
	); err != nil {
		t.Fatalf("encode value within retained constraint: %v", err)
	}
	if _, err := schema.EncodeCanonical(
		[]mib.IndexValue{mib.NewInteger32IndexValue(11)},
		mib.EncodeOptions{MaxSuffixArcs: 1},
	); err == nil {
		t.Fatal("value outside retained constraint should fail")
	}
}

func TestBoundIndexCodecCompleteOIDLimit(t *testing.T) {
	schema := compileSchema(t, "simpleEntry")
	bound, err := mib.NewBoundIndexCodec(schema, make(mib.OID, 127), mib.MaxInstanceOIDArcs)
	if err != nil {
		t.Fatalf("bind at 127 arcs: %v", err)
	}
	if bound.MaxSuffixArcs() != 1 {
		t.Fatalf("suffix budget = %d, want 1", bound.MaxSuffixArcs())
	}
	encoded, err := bound.EncodeCanonical([]mib.IndexValue{mib.NewInteger32IndexValue(1)}, mib.IncompleteConstraintAllow)
	if err != nil || encoded.Suffix().Len() != 1 {
		t.Fatalf("128-arc complete instance: %#v, %v", encoded, err)
	}
	if _, err := mib.NewBoundIndexCodec(schema, make(mib.OID, 128), mib.MaxInstanceOIDArcs); err == nil {
		t.Fatal("schema minimum should not fit after a 128-arc object OID")
	}
	if _, err := mib.NewBoundIndexCodec(schema, make(mib.OID, 129), mib.MaxInstanceOIDArcs); err == nil {
		t.Fatal("129-arc object OID should be rejected")
	}
}
