package model

import "testing"

func hasIndexSchemaIssue(issues []IndexSchemaIssue, code IndexSchemaIssueCode) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestCompileIndexSchemaRetainsZeroWidthIssues(t *testing.T) {
	_, row, zeroColumn, integerColumn := buildTableTree()
	zeroColumn.typ = &Type{base: BaseOctetString}
	zeroColumn.sizes = []Range{{Min: NewUnsignedRangeBound(0), Max: NewUnsignedRangeBound(0)}}
	zeroColumn.sizesConstrained = true
	integerColumn.typ = &Type{base: BaseInteger32}

	t.Run("complete index", func(t *testing.T) {
		row.index = []IndexEntry{{Object: zeroColumn}}
		schema, err := CompileIndexSchema(row)
		if err != nil {
			t.Fatalf("CompileIndexSchema: %v", err)
		}
		if schema.MinimumWidth() != 0 {
			t.Fatalf("MinimumWidth = %d, want 0", schema.MinimumWidth())
		}
		maximum, known := schema.MaximumWidth()
		if maximum != 0 || !known {
			t.Fatalf("MaximumWidth = (%d, %v), want (0, true)", maximum, known)
		}
		if !hasIndexSchemaIssue(schema.Component(0).Issues(), IndexSchemaZeroWidthComponent) {
			t.Fatalf("component issues = %#v", schema.Component(0).Issues())
		}
		if !hasIndexSchemaIssue(schema.Issues(), IndexSchemaZeroWidthIndex) {
			t.Fatalf("schema issues = %#v", schema.Issues())
		}

		encoded, err := schema.EncodeCanonical(
			[]IndexValue{NewOctetStringIndexValue(nil)},
			EncodeOptions{MaxSuffixArcs: 0},
		)
		if err != nil {
			t.Fatalf("EncodeCanonical: %v", err)
		}
		if encoded.Suffix().Len() != 0 {
			t.Fatalf("encoded suffix = %v, want empty", encoded.Suffix().Arcs())
		}
		decoded, err := schema.DecodeExact(nil, DecodeOptions{MaxSuffixArcs: 0})
		if err != nil {
			t.Fatalf("DecodeExact: %v", err)
		}
		components := decoded.Components()
		if len(components) != 1 || components[0].Offset() != 0 || len(components[0].EncodedArcs()) != 0 {
			t.Fatalf("decoded components = %#v", components)
		}
		if _, err := NewBoundIndexCodec(schema, OID{1, 3, 6}, 3); err != nil {
			t.Fatalf("NewBoundIndexCodec with zero suffix budget: %v", err)
		}
	})

	t.Run("composite index", func(t *testing.T) {
		row.index = []IndexEntry{{Object: zeroColumn}, {Object: integerColumn}}
		schema, err := CompileIndexSchema(row)
		if err != nil {
			t.Fatalf("CompileIndexSchema: %v", err)
		}
		if !hasIndexSchemaIssue(schema.Component(0).Issues(), IndexSchemaZeroWidthComponent) {
			t.Fatalf("component issues = %#v", schema.Component(0).Issues())
		}
		if hasIndexSchemaIssue(schema.Issues(), IndexSchemaZeroWidthIndex) {
			t.Fatalf("schema issues = %#v, unexpected zero-width index", schema.Issues())
		}

		encoded, err := schema.EncodeCanonical(
			[]IndexValue{NewOctetStringIndexValue(nil), NewInteger32IndexValue(7)},
			EncodeOptions{MaxSuffixArcs: 1},
		)
		if err != nil {
			t.Fatalf("EncodeCanonical: %v", err)
		}
		if got := encoded.Suffix().Arcs(); len(got) != 1 || got[0] != 7 {
			t.Fatalf("encoded suffix = %v, want [7]", got)
		}
		decoded, err := schema.DecodeExact(OID{7}, DecodeOptions{MaxSuffixArcs: 1})
		if err != nil {
			t.Fatalf("DecodeExact: %v", err)
		}
		components := decoded.Components()
		if len(components) != 2 || components[0].Offset() != 0 || components[1].Offset() != 0 || len(components[0].EncodedArcs()) != 0 {
			t.Fatalf("decoded components = %#v", components)
		}
	})
}

func TestCompileIndexSchemaRetainsImpliedEmptyIssue(t *testing.T) {
	_, row, column, _ := buildTableTree()
	typ := &Type{base: BaseOctetString}
	column.typ = typ
	column.sizes = []Range{{Min: NewUnsignedRangeBound(0), Max: NewUnsignedRangeBound(32)}}
	column.sizesConstrained = true
	row.index = []IndexEntry{{Object: column, Implied: true}}

	schema, err := CompileIndexSchema(row)
	if err != nil {
		t.Fatalf("CompileIndexSchema: %v", err)
	}
	issues := schema.Component(0).Issues()
	if len(issues) != 1 || issues[0].Code != IndexSchemaImpliedOctetsMayBeEmpty {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestIncompleteConstraintEnforcement(t *testing.T) {
	constraint, _ := normalizeConstraint([]Range{{Min: NewUnsignedRangeBound(5), Max: NewRawRangeBound("vendor-max")}}, true, 100)
	if constraint.Check(4) != ConstraintInvalid || constraint.Check(5) != ConstraintIndeterminate {
		t.Fatalf("unexpected incomplete checks: 4=%v 5=%v", constraint.Check(4), constraint.Check(5))
	}
	minimum, _, maximumKnown := constraintWidths(constraint)
	if minimum != 5 || maximumKnown {
		t.Fatalf("incomplete widths = min %d, maxKnown %v", minimum, maximumKnown)
	}
}

func TestObjectIdentifierLengthConstraintIsRetained(t *testing.T) {
	_, row, column, _ := buildTableTree()
	column.typ = &Type{base: BaseObjectIdentifier}
	column.sizes = []Range{{Min: NewUnsignedRangeBound(2), Max: NewUnsignedRangeBound(4)}}
	column.sizesConstrained = true
	row.index = []IndexEntry{{Object: column}}

	schema, err := CompileIndexSchema(row)
	if err != nil {
		t.Fatalf("CompileIndexSchema: %v", err)
	}
	wire := schema.Component(0).WireType()
	if wire.Framing() != IndexFramingLengthPrefixed || wire.Constraint().Check(1) != ConstraintInvalid || wire.Constraint().Check(3) != ConstraintValid {
		t.Fatalf("OBJECT IDENTIFIER wire metadata = %#v", wire)
	}
	if _, err := schema.DecodeExact(OID{1, 42}, DecodeOptions{MaxSuffixArcs: 2}); err == nil {
		t.Fatal("known-invalid OBJECT IDENTIFIER length was accepted")
	}
}
