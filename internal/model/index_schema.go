package model

import (
	"fmt"
	"math"
	"slices"
	"sort"
)

// ConstraintState distinguishes absent, complete, incomplete, and empty
// effective constraints.
type ConstraintState uint8

const (
	ConstraintUnspecified ConstraintState = iota
	ConstraintKnown
	ConstraintIncomplete
	ConstraintEmpty
)

// ConstraintResult is the result of checking a value against owned metadata.
type ConstraintResult uint8

const (
	ConstraintValid ConstraintResult = iota
	ConstraintInvalid
	ConstraintIndeterminate
)

// ConstraintMode controls whether exact structural operations reject known
// MIB constraint violations or return them with the result.
type ConstraintMode uint8

const (
	ConstraintEnforce ConstraintMode = iota
	ConstraintReport
)

// IncompleteConstraintMode controls canonical encoding when unresolved
// metadata cannot prove a value valid or invalid.
type IncompleteConstraintMode uint8

const (
	IncompleteConstraintAllow IncompleteConstraintMode = iota
	IncompleteConstraintReject
)

// IndexRange is a normalized inclusive non-negative range.
type IndexRange struct {
	Min uint64
	Max uint64
}

// IncompleteIndexRange is an alternative with at least one unresolved bound.
// Nil bounds are unknown rather than open-ended declarations.
type IncompleteIndexRange struct {
	Min *uint64
	Max *uint64
}

// IndexConstraint is an owned normalized constraint. Concrete alternatives are
// sorted, merged, and disjoint.
type IndexConstraint struct {
	state      ConstraintState
	ranges     []IndexRange
	incomplete []IncompleteIndexRange
}

func (c IndexConstraint) State() ConstraintState { return c.state }
func (c IndexConstraint) Ranges() []IndexRange   { return slices.Clone(c.ranges) }
func (c IndexConstraint) IncompleteRanges() []IncompleteIndexRange {
	result := make([]IncompleteIndexRange, len(c.incomplete))
	for i, r := range c.incomplete {
		result[i] = IncompleteIndexRange{Min: cloneUint64(r.Min), Max: cloneUint64(r.Max)}
	}
	return result
}

func cloneUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// Check reports whether value is accepted, rejected, or cannot be decided
// because an unresolved alternative may contain it.
func (c IndexConstraint) Check(value uint64) ConstraintResult {
	switch c.state {
	case ConstraintUnspecified:
		return ConstraintValid
	case ConstraintEmpty:
		return ConstraintInvalid
	}
	for _, r := range c.ranges {
		if value >= r.Min && value <= r.Max {
			return ConstraintValid
		}
	}
	for _, r := range c.incomplete {
		if r.Min != nil && value < *r.Min {
			continue
		}
		if r.Max != nil && value > *r.Max {
			continue
		}
		return ConstraintIndeterminate
	}
	return ConstraintInvalid
}

func (c IndexConstraint) fixedValue() (uint64, bool) {
	returnValue := uint64(0)
	if c.state == ConstraintKnown && len(c.ranges) == 1 && c.ranges[0].Min == c.ranges[0].Max {
		returnValue = c.ranges[0].Min
		return returnValue, true
	}
	return 0, false
}

// IndexFraming identifies how a variable-width component is placed in a suffix.
type IndexFraming uint8

const (
	IndexFramingNone IndexFraming = iota
	IndexFramingFixed
	IndexFramingLengthPrefixed
	IndexFramingImplied
)

// IndexWireKind identifies the algebraic wire family of a schema component.
type IndexWireKind uint8

const (
	IndexWireInteger IndexWireKind = iota + 1
	IndexWireIpAddress
	IndexWireOctets
	IndexWireObjectIdentifier
)

// IntegerIndexKind preserves the precise semantic integer type.
type IntegerIndexKind uint8

const (
	IntegerIndexInteger32 IntegerIndexKind = iota + 1
	IntegerIndexUnsigned32
	IntegerIndexGauge32
	IntegerIndexTimeTicks
	IntegerIndexCounter32
)

// OctetIndexKind preserves the precise octet-valued semantic type.
type OctetIndexKind uint8

const (
	OctetIndexOctetString OctetIndexKind = iota + 1
	OctetIndexBits
	OctetIndexOpaque
)

// IndexWireType is an immutable-by-convention wire description. Accessors
// return cloned slices where necessary.
type IndexWireType struct {
	kind        IndexWireKind
	integerKind IntegerIndexKind
	octetKind   OctetIndexKind
	framing     IndexFraming
	fixedWidth  int
	constraint  IndexConstraint
	enums       []uint32
}

func (w IndexWireType) Kind() IndexWireKind           { return w.kind }
func (w IndexWireType) IntegerKind() IntegerIndexKind { return w.integerKind }
func (w IndexWireType) OctetKind() OctetIndexKind     { return w.octetKind }
func (w IndexWireType) Framing() IndexFraming         { return w.framing }
func (w IndexWireType) FixedWidth() (int, bool)       { return w.fixedWidth, w.framing == IndexFramingFixed }
func (w IndexWireType) Constraint() IndexConstraint   { return cloneIndexConstraint(w.constraint) }
func (w IndexWireType) EnumValues() []uint32          { return slices.Clone(w.enums) }

func cloneIndexConstraint(c IndexConstraint) IndexConstraint {
	return IndexConstraint{state: c.state, ranges: slices.Clone(c.ranges), incomplete: c.IncompleteRanges()}
}

// IndexSchemaIssueCode identifies a representable MIB deviation or schema concern.
type IndexSchemaIssueCode uint8

const (
	IndexSchemaCounter32Extension IndexSchemaIssueCode = iota + 1
	IndexSchemaUnrepresentableConstraint
	IndexSchemaUnrepresentableEnum
	IndexSchemaImpliedOctetsMayBeEmpty
	IndexSchemaUnresolvedObjectIdentity
	IndexSchemaZeroWidthComponent
	IndexSchemaZeroWidthIndex
)

// IndexSchemaIssue is retained with an owned schema after the MIB is released.
type IndexSchemaIssue struct {
	Code    IndexSchemaIssueCode
	Message string
}

// IndexComponentSchema is one owned effective INDEX component.
type IndexComponentSchema struct {
	name      string
	objectOID OID
	wireType  IndexWireType
	issues    []IndexSchemaIssue
}

func (c *IndexComponentSchema) Name() string               { return c.name }
func (c *IndexComponentSchema) ObjectOID() OID             { return slices.Clone(c.objectOID) }
func (c *IndexComponentSchema) WireType() IndexWireType    { return cloneWireType(c.wireType) }
func (c *IndexComponentSchema) Issues() []IndexSchemaIssue { return slices.Clone(c.issues) }

func cloneWireType(w IndexWireType) IndexWireType {
	w.constraint = cloneIndexConstraint(w.constraint)
	w.enums = slices.Clone(w.enums)
	return w
}

// IndexSchema owns all metadata needed to decode and encode one effective row key.
type IndexSchema struct {
	components []IndexComponentSchema
	minWidth   int
	maxWidth   int
	maxKnown   bool
	issues     []IndexSchemaIssue
}

// MaxInstanceOIDArcs is the SMI limit for a complete instance identifier.
const MaxInstanceOIDArcs = 128

// BoundIndexCodec binds a reusable row schema to one destination OID budget.
type BoundIndexCodec struct {
	schema        *IndexSchema
	maxSuffixArcs int
}

// NewBoundIndexCodec derives a suffix budget from a destination object OID.
// maxInstanceArcs may be smaller than the SMI maximum for application limits.
func NewBoundIndexCodec(schema *IndexSchema, objectOID OID, maxInstanceArcs int) (*BoundIndexCodec, error) {
	if schema == nil {
		return nil, &IndexSchemaError{Reason: IndexSchemaInvalidObject, Position: -1, Detail: "nil index schema"}
	}
	if maxInstanceArcs < 0 || maxInstanceArcs > MaxInstanceOIDArcs || len(objectOID) > maxInstanceArcs {
		return nil, &IndexSchemaError{Reason: IndexSchemaInvalidObject, Position: -1, Detail: "object OID exceeds the complete instance limit"}
	}
	maximum := maxInstanceArcs - len(objectOID)
	if schema.minWidth > maximum {
		return nil, &IndexSchemaError{Reason: IndexSchemaInvalidObject, Position: -1, Detail: "minimum index suffix does not fit the complete instance limit"}
	}
	return &BoundIndexCodec{schema: schema, maxSuffixArcs: maximum}, nil
}

// BindIndexSchema binds a schema to an object's OID using the 128-arc SMI limit.
func BindIndexSchema(schema *IndexSchema, object *Object) (*BoundIndexCodec, error) {
	if object == nil || len(object.OID()) == 0 {
		return nil, &IndexSchemaError{Reason: IndexSchemaInvalidObject, Position: -1, Detail: "destination object has no resolved OID"}
	}
	return NewBoundIndexCodec(schema, object.OID(), MaxInstanceOIDArcs)
}

func (b *BoundIndexCodec) Schema() *IndexSchema { return b.schema }
func (b *BoundIndexCodec) MaxSuffixArcs() int   { return b.maxSuffixArcs }
func (b *BoundIndexCodec) DecodeExact(suffix OID, mode ConstraintMode) (*DecodedRowIndex, error) {
	return b.schema.DecodeExact(suffix, DecodeOptions{MaxSuffixArcs: b.maxSuffixArcs, ConstraintMode: mode})
}
func (b *BoundIndexCodec) EncodeCanonical(values []IndexValue, incomplete IncompleteConstraintMode) (*EncodedIndex, error) {
	return b.schema.EncodeCanonical(values, EncodeOptions{MaxSuffixArcs: b.maxSuffixArcs, IncompleteConstraint: incomplete})
}

func (s *IndexSchema) Len() int { return len(s.components) }
func (s *IndexSchema) Component(position int) *IndexComponentSchema {
	if position < 0 || position >= len(s.components) {
		return nil
	}
	return &s.components[position]
}
func (s *IndexSchema) Components() []IndexComponentSchema {
	result := make([]IndexComponentSchema, len(s.components))
	for i := range s.components {
		result[i] = IndexComponentSchema{
			name: s.components[i].name, objectOID: slices.Clone(s.components[i].objectOID),
			wireType: cloneWireType(s.components[i].wireType), issues: slices.Clone(s.components[i].issues),
		}
	}
	return result
}
func (s *IndexSchema) MinimumWidth() int          { return s.minWidth }
func (s *IndexSchema) MaximumWidth() (int, bool)  { return s.maxWidth, s.maxKnown }
func (s *IndexSchema) Issues() []IndexSchemaIssue { return slices.Clone(s.issues) }

// IndexSchemaErrorReason identifies why resolved metadata could not define a codec.
type IndexSchemaErrorReason uint8

const (
	IndexSchemaNoIndexes IndexSchemaErrorReason = iota + 1
	IndexSchemaInvalidObject
	IndexSchemaUnresolvedType
	IndexSchemaUnsupportedType
	IndexSchemaInvalidImplied
	IndexSchemaEmptyConstraint
	IndexSchemaOverflow
)

// IndexSchemaError is returned when safe deterministic schema compilation fails.
type IndexSchemaError struct {
	Reason   IndexSchemaErrorReason
	Position int
	Name     string
	Detail   string
}

func (e *IndexSchemaError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("index schema component %d %q: %s", e.Position, e.Name, e.Detail)
	}
	return "index schema: " + e.Detail
}

// CompileIndexSchema copies and normalizes the effective INDEX metadata of a row or column.
func CompileIndexSchema(object *Object) (*IndexSchema, error) {
	if object == nil || (!object.IsRow() && !object.IsColumn()) {
		return nil, &IndexSchemaError{Reason: IndexSchemaInvalidObject, Position: -1, Detail: "object is not a row or column"}
	}
	indexes := object.EffectiveIndexes()
	if len(indexes) == 0 {
		return nil, &IndexSchemaError{Reason: IndexSchemaNoIndexes, Position: -1, Detail: "no effective INDEX clause"}
	}
	schema := &IndexSchema{components: make([]IndexComponentSchema, 0, len(indexes)), maxKnown: true}
	for position, entry := range indexes {
		component, err := compileIndexComponent(entry, position, len(indexes))
		if err != nil {
			return nil, err
		}
		min, max, known := componentWidth(component.wireType)
		if schema.minWidth > math.MaxInt-min || (known && schema.maxWidth > math.MaxInt-max) {
			return nil, &IndexSchemaError{Reason: IndexSchemaOverflow, Position: position, Name: component.name, Detail: "encoded width overflow"}
		}
		schema.minWidth += min
		if schema.maxKnown && known {
			schema.maxWidth += max
		} else {
			schema.maxKnown = false
		}
		schema.components = append(schema.components, component)
	}
	if schema.maxKnown && schema.maxWidth == 0 {
		schema.issues = append(schema.issues, IndexSchemaIssue{
			Code:    IndexSchemaZeroWidthIndex,
			Message: "complete effective index consumes no OID arcs and cannot identify rows",
		})
	}
	return schema, nil
}

func compileIndexComponent(entry IndexEntry, position, count int) (IndexComponentSchema, error) {
	name := entry.TypeName
	typ := entry.Type
	var sizes, ranges []Range
	var sizesConstrained, rangesConstrained bool
	var enums []NamedValue
	var objectOID OID
	if entry.Object != nil {
		name = entry.Object.Name()
		typ = entry.Object.Type()
		sizes, ranges = entry.Object.EffectiveSizes(), entry.Object.EffectiveRanges()
		sizesConstrained, rangesConstrained = entry.Object.EffectiveSizesConstrained(), entry.Object.EffectiveRangesConstrained()
		enums = entry.Object.EffectiveEnums()
		objectOID = entry.Object.OID()
	} else if typ != nil {
		sizes, ranges = typ.EffectiveSizes(), typ.EffectiveRanges()
		sizesConstrained, rangesConstrained = typ.EffectiveSizesConstrained(), typ.EffectiveRangesConstrained()
		enums = typ.EffectiveEnums()
	}
	if name == "" {
		name = "<unresolved>"
	}
	if typ == nil || typ.EffectiveBase() == BaseUnknown {
		return IndexComponentSchema{}, schemaComponentError(IndexSchemaUnresolvedType, position, name, "effective type is unresolved")
	}
	if entry.Implied && position != count-1 {
		return IndexComponentSchema{}, schemaComponentError(IndexSchemaInvalidImplied, position, name, "IMPLIED component is not last")
	}

	component := IndexComponentSchema{name: name, objectOID: slices.Clone(objectOID)}
	if entry.Object != nil && len(objectOID) == 0 {
		component.issues = append(component.issues, IndexSchemaIssue{Code: IndexSchemaUnresolvedObjectIdentity, Message: "referenced index object has no resolved numeric OID"})
	}
	base := typ.EffectiveBase()
	switch base {
	case BaseInteger32, BaseUnsigned32, BaseGauge32, BaseTimeTicks, BaseCounter32:
		if entry.Implied {
			return IndexComponentSchema{}, schemaComponentError(IndexSchemaInvalidImplied, position, name, "IMPLIED requires a variable-length value")
		}
		max := uint64(math.MaxUint32)
		kind := IntegerIndexUnsigned32
		switch base {
		case BaseInteger32:
			kind, max = IntegerIndexInteger32, uint64(math.MaxInt32)
		case BaseGauge32:
			kind = IntegerIndexGauge32
		case BaseTimeTicks:
			kind = IntegerIndexTimeTicks
		case BaseCounter32:
			kind = IntegerIndexCounter32
			component.issues = append(component.issues, IndexSchemaIssue{Code: IndexSchemaCounter32Extension, Message: "Counter32 is not a conforming INDEX type"})
		}
		constraint, dropped := normalizeConstraint(ranges, rangesConstrained, max)
		if constraint.state == ConstraintEmpty {
			return IndexComponentSchema{}, schemaComponentError(IndexSchemaEmptyConstraint, position, name, "no representable integer value remains")
		}
		if dropped {
			component.issues = append(component.issues, IndexSchemaIssue{Code: IndexSchemaUnrepresentableConstraint, Message: "constraint contains values outside the wire domain"})
		}
		enumValues, enumDropped := normalizeEnums(enums, max)
		if len(enumValues) > 0 && constraint.state != ConstraintUnspecified {
			filtered := enumValues[:0]
			for _, value := range enumValues {
				if constraint.Check(uint64(value)) != ConstraintInvalid {
					filtered = append(filtered, value)
				} else {
					enumDropped = true
				}
			}
			enumValues = filtered
		}
		if len(enums) > 0 && len(enumValues) == 0 {
			return IndexComponentSchema{}, schemaComponentError(IndexSchemaEmptyConstraint, position, name, "no representable enumeration value remains")
		}
		if enumDropped {
			component.issues = append(component.issues, IndexSchemaIssue{Code: IndexSchemaUnrepresentableEnum, Message: "enumeration contains values outside the wire domain"})
		}
		component.wireType = IndexWireType{kind: IndexWireInteger, integerKind: kind, framing: IndexFramingFixed, fixedWidth: 1, constraint: constraint, enums: enumValues}

	case BaseIpAddress:
		if entry.Implied {
			return IndexComponentSchema{}, schemaComponentError(IndexSchemaInvalidImplied, position, name, "IMPLIED requires a variable-length value")
		}
		component.wireType = IndexWireType{kind: IndexWireIpAddress, framing: IndexFramingFixed, fixedWidth: 4, constraint: IndexConstraint{state: ConstraintUnspecified}}

	case BaseOctetString, BaseBits, BaseOpaque:
		constraint, dropped := normalizeConstraint(sizes, sizesConstrained, math.MaxUint32)
		if constraint.state == ConstraintEmpty {
			return IndexComponentSchema{}, schemaComponentError(IndexSchemaEmptyConstraint, position, name, "no representable length remains")
		}
		if dropped {
			component.issues = append(component.issues, IndexSchemaIssue{Code: IndexSchemaUnrepresentableConstraint, Message: "SIZE contains values outside the wire domain"})
		}
		framing, fixedWidth := IndexFramingLengthPrefixed, 0
		if fixed, ok := constraint.fixedValue(); ok {
			if fixed > math.MaxInt {
				return IndexComponentSchema{}, schemaComponentError(IndexSchemaOverflow, position, name, "fixed SIZE exceeds platform width")
			}
			framing, fixedWidth = IndexFramingFixed, int(fixed)
		}
		if framing == IndexFramingFixed && fixedWidth == 0 {
			component.issues = append(component.issues, IndexSchemaIssue{
				Code:    IndexSchemaZeroWidthComponent,
				Message: "fixed-width component consumes no OID arcs",
			})
		}
		if entry.Implied {
			if framing == IndexFramingFixed {
				return IndexComponentSchema{}, schemaComponentError(IndexSchemaInvalidImplied, position, name, "IMPLIED value is fixed-width")
			}
			if constraint.Check(0) != ConstraintInvalid {
				component.issues = append(component.issues, IndexSchemaIssue{Code: IndexSchemaImpliedOctetsMayBeEmpty, Message: "IMPLIED octet value may be empty"})
			}
			framing = IndexFramingImplied
		}
		octetKind := OctetIndexOctetString
		if base == BaseBits {
			octetKind = OctetIndexBits
		} else if base == BaseOpaque {
			octetKind = OctetIndexOpaque
		}
		component.wireType = IndexWireType{kind: IndexWireOctets, octetKind: octetKind, framing: framing, fixedWidth: fixedWidth, constraint: constraint}

	case BaseObjectIdentifier:
		constraint, dropped := normalizeConstraint(sizes, sizesConstrained, math.MaxUint32)
		if constraint.state == ConstraintEmpty {
			return IndexComponentSchema{}, schemaComponentError(IndexSchemaEmptyConstraint, position, name, "no representable OBJECT IDENTIFIER length remains")
		}
		if dropped {
			component.issues = append(component.issues, IndexSchemaIssue{Code: IndexSchemaUnrepresentableConstraint, Message: "SIZE contains values outside the wire domain"})
		}
		framing := IndexFramingLengthPrefixed
		if entry.Implied {
			framing = IndexFramingImplied
		}
		component.wireType = IndexWireType{kind: IndexWireObjectIdentifier, framing: framing, constraint: constraint}

	default:
		return IndexComponentSchema{}, schemaComponentError(IndexSchemaUnsupportedType, position, name, "unsupported effective base type "+base.String())
	}
	return component, nil
}

func schemaComponentError(reason IndexSchemaErrorReason, position int, name, detail string) error {
	return &IndexSchemaError{Reason: reason, Position: position, Name: name, Detail: detail}
}

func normalizeEnums(values []NamedValue, maximum uint64) ([]uint32, bool) {
	result := make([]uint32, 0, len(values))
	dropped := false
	for _, value := range values {
		if value.Value < 0 || uint64(value.Value) > maximum {
			dropped = true
			continue
		}
		result = append(result, uint32(value.Value))
	}
	slices.Sort(result)
	result = slices.Compact(result)
	return result, dropped
}

func normalizeConstraint(source []Range, constrained bool, maximum uint64) (IndexConstraint, bool) {
	if !constrained {
		return IndexConstraint{state: ConstraintUnspecified}, false
	}
	constraint := IndexConstraint{}
	dropped := false
	for _, sourceRange := range source {
		min, minKnown, minOutside := normalizedBound(sourceRange.Min, true, maximum)
		max, maxKnown, maxOutside := normalizedBound(sourceRange.Max, false, maximum)
		dropped = dropped || minOutside || maxOutside
		if (minKnown && min > maximum) || (maxKnown && max > maximum) || (minKnown && maxKnown && min > max) {
			continue
		}
		if minKnown && maxKnown {
			constraint.ranges = append(constraint.ranges, IndexRange{Min: min, Max: max})
		} else {
			incomplete := IncompleteIndexRange{}
			if minKnown {
				incomplete.Min = cloneUint64(&min)
			}
			if maxKnown {
				incomplete.Max = cloneUint64(&max)
			}
			constraint.incomplete = append(constraint.incomplete, incomplete)
		}
	}
	constraint.ranges = mergeIndexRanges(constraint.ranges)
	switch {
	case len(constraint.incomplete) > 0:
		constraint.state = ConstraintIncomplete
	case len(constraint.ranges) > 0:
		constraint.state = ConstraintKnown
	default:
		constraint.state = ConstraintEmpty
	}
	return constraint, dropped
}

func normalizedBound(bound RangeBound, lower bool, maximum uint64) (uint64, bool, bool) {
	switch bound.Kind {
	case RangeBoundSigned:
		if bound.Signed < 0 {
			if lower {
				return 0, true, true
			}
			return maximum + boolToUint64(maximum < math.MaxUint64), true, true
		}
		value := uint64(bound.Signed)
		if value > maximum {
			if lower {
				return value, true, true
			}
			return maximum, true, true
		}
		return value, true, false
	case RangeBoundUnsigned:
		if bound.Unsigned > maximum {
			if lower {
				return bound.Unsigned, true, true
			}
			return maximum, true, true
		}
		return bound.Unsigned, true, false
	case RangeBoundMin:
		if lower {
			return 0, true, false
		}
	case RangeBoundMax:
		if !lower {
			return maximum, true, false
		}
	}
	return 0, false, false
}

func boolToUint64(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

func mergeIndexRanges(ranges []IndexRange) []IndexRange {
	if len(ranges) < 2 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Min < ranges[j].Min || (ranges[i].Min == ranges[j].Min && ranges[i].Max < ranges[j].Max)
	})
	result := ranges[:1]
	for _, current := range ranges[1:] {
		last := &result[len(result)-1]
		adjacent := last.Max != math.MaxUint64 && current.Min == last.Max+1
		if current.Min <= last.Max || adjacent {
			if current.Max > last.Max {
				last.Max = current.Max
			}
			continue
		}
		result = append(result, current)
	}
	return result
}

func componentWidth(wire IndexWireType) (int, int, bool) {
	switch wire.framing {
	case IndexFramingFixed:
		return wire.fixedWidth, wire.fixedWidth, true
	case IndexFramingLengthPrefixed:
		minimum, maximum, known := constraintWidths(wire.constraint)
		return minimum + 1, maximum + 1, known
	case IndexFramingImplied:
		return constraintWidths(wire.constraint)
	default:
		return 0, 0, false
	}
}

func constraintWidths(constraint IndexConstraint) (int, int, bool) {
	if constraint.state == ConstraintKnown && len(constraint.ranges) > 0 {
		min, max := constraint.ranges[0].Min, constraint.ranges[0].Max
		for _, r := range constraint.ranges[1:] {
			if r.Min < min {
				min = r.Min
			}
			if r.Max > max {
				max = r.Max
			}
		}
		if min <= math.MaxInt && max <= math.MaxInt {
			return int(min), int(max), true
		}
	}
	if constraint.state == ConstraintIncomplete {
		minimum, maximum := uint64(math.MaxUint64), uint64(0)
		maximumKnown := true
		for _, r := range constraint.ranges {
			minimum = min(minimum, r.Min)
			maximum = max(maximum, r.Max)
		}
		for _, r := range constraint.incomplete {
			if r.Min == nil {
				minimum = 0
			} else {
				minimum = min(minimum, *r.Min)
			}
			if r.Max == nil {
				maximumKnown = false
			} else {
				maximum = max(maximum, *r.Max)
			}
		}
		if minimum == math.MaxUint64 {
			minimum = 0
		}
		if minimum <= math.MaxInt && maximum <= math.MaxInt {
			return int(minimum), int(maximum), maximumKnown
		}
	}
	return 0, 0, false
}

// DecodeOptions bounds one exact decode operation.
type DecodeOptions struct {
	MaxSuffixArcs  int
	ConstraintMode ConstraintMode
}

// EncodeOptions bounds one canonical encoding operation.
type EncodeOptions struct {
	MaxSuffixArcs        int
	IncompleteConstraint IncompleteConstraintMode
}

// IndexConstraintViolation describes a known or indeterminate MIB metadata conflict.
type IndexConstraintViolation struct {
	Position      int
	Name          string
	Value         uint64
	Indeterminate bool
	Detail        string
}

// DecodedIndexComponent borrows the exact arcs from the decode input.
type DecodedIndexComponent struct {
	component *IndexComponentSchema
	value     IndexValue
	encoded   OID
	offset    int
}

func (d DecodedIndexComponent) Schema() *IndexComponentSchema { return d.component }
func (d DecodedIndexComponent) Value() IndexValue             { return cloneIndexValue(d.value) }
func (d DecodedIndexComponent) EncodedArcs() OID              { return d.encoded }
func (d DecodedIndexComponent) Offset() int                   { return d.offset }

// DecodedRowIndex is an exact structural decode result.
type DecodedRowIndex struct {
	components []DecodedIndexComponent
	violations []IndexConstraintViolation
}

func (d *DecodedRowIndex) Components() []DecodedIndexComponent { return slices.Clone(d.components) }
func (d *DecodedRowIndex) Values() []IndexValue {
	result := make([]IndexValue, len(d.components))
	for i := range d.components {
		result[i] = cloneIndexValue(d.components[i].value)
	}
	return result
}
func (d *DecodedRowIndex) Violations() []IndexConstraintViolation { return slices.Clone(d.violations) }

// IndexDecodeErrorReason identifies an exact structural or enforced failure.
type IndexDecodeErrorReason uint8

const (
	IndexDecodeInvalidLimit IndexDecodeErrorReason = iota + 1
	IndexDecodeSuffixTooLong
	IndexDecodeTruncated
	IndexDecodeLengthExceedsLimit
	IndexDecodeInvalidOctet
	IndexDecodePrimitiveDomain
	IndexDecodeTrailingArcs
	IndexDecodeConstraintViolation
)

// IndexDecodeError retains the decoded prefix and unconsumed input.
type IndexDecodeError struct {
	Reason    IndexDecodeErrorReason
	Position  int
	Name      string
	Offset    int
	Needed    int
	Available int
	Value     uint64
	Decoded   []DecodedIndexComponent
	Remaining OID
	Detail    string
}

func (e *IndexDecodeError) Error() string { return "index decode: " + e.Detail }

// DecodeExact decodes every schema component and consumes every suffix arc.
func (s *IndexSchema) DecodeExact(suffix OID, options DecodeOptions) (*DecodedRowIndex, error) {
	if options.MaxSuffixArcs < 0 {
		return nil, &IndexDecodeError{Reason: IndexDecodeInvalidLimit, Position: -1, Detail: "negative suffix limit"}
	}
	if len(suffix) > options.MaxSuffixArcs {
		return nil, &IndexDecodeError{Reason: IndexDecodeSuffixTooLong, Position: -1, Available: len(suffix), Needed: options.MaxSuffixArcs, Remaining: suffix, Detail: "suffix exceeds operation limit"}
	}
	result := &DecodedRowIndex{components: make([]DecodedIndexComponent, 0, len(s.components))}
	position := 0
	for i := range s.components {
		component := &s.components[i]
		start := position
		wire := component.wireType
		var value IndexValue
		var valueLength uint64
		switch wire.framing {
		case IndexFramingFixed:
			if len(suffix)-position < wire.fixedWidth {
				return nil, decodeFailure(IndexDecodeTruncated, i, component.name, position, wire.fixedWidth, len(suffix)-position, result, suffix[position:], "fixed component is truncated")
			}
			data := suffix[position : position+wire.fixedWidth]
			if wire.kind == IndexWireInteger && wire.integerKind == IntegerIndexInteger32 && data[0] > math.MaxInt32 {
				failure := decodeFailure(IndexDecodePrimitiveDomain, i, component.name, position, 0, 0, result, suffix[position:], "Integer32 arc exceeds MaxInt32")
				failure.Value = uint64(data[0])
				return nil, failure
			}
			if offset, invalid := invalidOctetArc(wire, data); invalid {
				failure := decodeFailure(IndexDecodeInvalidOctet, i, component.name, position+offset, 0, 0, result, suffix[position:], "octet value contains a non-octet arc")
				failure.Value = uint64(data[offset])
				return nil, failure
			}
			var err error
			value, err = decodeWireValue(wire, data)
			if err != nil {
				return nil, decodeFailure(IndexDecodeInvalidOctet, i, component.name, position, 0, 0, result, suffix[position:], err.Error())
			}
			position += wire.fixedWidth
			valueLength = uint64(wire.fixedWidth)
		case IndexFramingLengthPrefixed:
			if position >= len(suffix) {
				return nil, decodeFailure(IndexDecodeTruncated, i, component.name, position, 1, 0, result, suffix[position:], "length prefix is missing")
			}
			length := uint64(suffix[position])
			if length > uint64(options.MaxSuffixArcs) || length > uint64(math.MaxInt) {
				return nil, decodeFailure(IndexDecodeLengthExceedsLimit, i, component.name, position, int(length), options.MaxSuffixArcs, result, suffix[position:], "component length exceeds operation limit")
			}
			available := len(suffix) - position - 1
			if int(length) > available {
				return nil, decodeFailure(IndexDecodeTruncated, i, component.name, position, int(length)+1, available+1, result, suffix[position:], "length-prefixed component is truncated")
			}
			data := suffix[position+1 : position+1+int(length)]
			if offset, invalid := invalidOctetArc(wire, data); invalid {
				failure := decodeFailure(IndexDecodeInvalidOctet, i, component.name, position+1+offset, 0, 0, result, suffix[position:], "octet value contains a non-octet arc")
				failure.Value = uint64(data[offset])
				return nil, failure
			}
			var err error
			value, err = decodeWireValue(wire, data)
			if err != nil {
				return nil, decodeFailure(IndexDecodeInvalidOctet, i, component.name, position+1, 0, 0, result, suffix[position:], err.Error())
			}
			position += 1 + int(length)
			valueLength = length
		case IndexFramingImplied:
			data := suffix[position:]
			if offset, invalid := invalidOctetArc(wire, data); invalid {
				failure := decodeFailure(IndexDecodeInvalidOctet, i, component.name, position+offset, 0, 0, result, suffix[position:], "octet value contains a non-octet arc")
				failure.Value = uint64(data[offset])
				return nil, failure
			}
			var err error
			value, err = decodeWireValue(wire, data)
			if err != nil {
				return nil, decodeFailure(IndexDecodeInvalidOctet, i, component.name, position, 0, 0, result, suffix[position:], err.Error())
			}
			position = len(suffix)
			valueLength = uint64(len(data))
		}
		if wire.kind == IndexWireInteger {
			valueLength = integerValue(value)
		}
		violations := componentViolations(i, component, valueLength)
		if violation := firstKnownViolation(violations); violation != nil && options.ConstraintMode == ConstraintEnforce {
			error := decodeFailure(IndexDecodeConstraintViolation, i, component.name, start, 0, 0, result, suffix[start:], violation.Detail)
			error.Value = valueLength
			return nil, error
		}
		result.violations = append(result.violations, violations...)
		result.components = append(result.components, DecodedIndexComponent{component: component, value: value, encoded: suffix[start:position], offset: start})
	}
	if position != len(suffix) {
		return nil, decodeFailure(IndexDecodeTrailingArcs, -1, "", position, 0, len(suffix)-position, result, suffix[position:], "trailing suffix arcs remain")
	}
	return result, nil
}

func invalidOctetArc(wire IndexWireType, data OID) (int, bool) {
	if wire.kind != IndexWireOctets && wire.kind != IndexWireIpAddress {
		return 0, false
	}
	for offset, arc := range data {
		if arc > math.MaxUint8 {
			return offset, true
		}
	}
	return 0, false
}

func firstKnownViolation(violations []IndexConstraintViolation) *IndexConstraintViolation {
	for i := range violations {
		if !violations[i].Indeterminate {
			return &violations[i]
		}
	}
	return nil
}

func decodeFailure(reason IndexDecodeErrorReason, position int, name string, offset, needed, available int, result *DecodedRowIndex, remaining OID, detail string) *IndexDecodeError {
	return &IndexDecodeError{Reason: reason, Position: position, Name: name, Offset: offset, Needed: needed, Available: available, Decoded: slices.Clone(result.components), Remaining: remaining, Detail: detail}
}

func decodeWireValue(wire IndexWireType, data OID) (IndexValue, error) {
	switch wire.kind {
	case IndexWireInteger:
		arc := data[0]
		switch wire.integerKind {
		case IntegerIndexInteger32:
			if arc > math.MaxInt32 {
				return IndexValue{}, fmt.Errorf("Integer32 arc exceeds MaxInt32")
			}
			return NewInteger32IndexValue(int32(arc)), nil
		case IntegerIndexUnsigned32:
			return NewUnsigned32IndexValue(arc), nil
		case IntegerIndexGauge32:
			return NewGauge32IndexValue(arc), nil
		case IntegerIndexTimeTicks:
			return NewTimeTicksIndexValue(arc), nil
		case IntegerIndexCounter32:
			return NewCounter32IndexValue(arc), nil
		}
	case IndexWireIpAddress:
		bytes, ok := arcsToBytes(data)
		if !ok {
			return IndexValue{}, fmt.Errorf("IP address contains a non-octet arc")
		}
		return NewIpAddressIndexValue([4]byte{bytes[0], bytes[1], bytes[2], bytes[3]}), nil
	case IndexWireOctets:
		bytes, ok := arcsToBytes(data)
		if !ok {
			return IndexValue{}, fmt.Errorf("octet value contains a non-octet arc")
		}
		switch wire.octetKind {
		case OctetIndexBits:
			return IndexValue{kind: IndexValueBits, value: bytes}, nil
		case OctetIndexOpaque:
			return IndexValue{kind: IndexValueOpaque, value: bytes}, nil
		default:
			return IndexValue{kind: IndexValueOctetString, value: bytes}, nil
		}
	case IndexWireObjectIdentifier:
		return NewObjectIdentifierIndexValue(data), nil
	}
	return IndexValue{}, fmt.Errorf("unsupported wire type")
}

func integerValue(value IndexValue) uint64 {
	if signed, ok := value.Signed(); ok {
		return uint64(signed)
	}
	unsigned, _ := value.Unsigned()
	return uint64(unsigned)
}

func componentViolations(position int, component *IndexComponentSchema, value uint64) []IndexConstraintViolation {
	result := component.wireType.constraint.Check(value)
	violations := make([]IndexConstraintViolation, 0, 2)
	if result != ConstraintValid {
		violations = append(violations, IndexConstraintViolation{Position: position, Name: component.name, Value: value, Indeterminate: result == ConstraintIndeterminate, Detail: "value conflicts with effective constraint"})
	}
	if len(component.wireType.enums) > 0 && !slices.Contains(component.wireType.enums, uint32(value)) {
		violations = append(violations, IndexConstraintViolation{Position: position, Name: component.name, Value: value, Detail: "value is not an effective enumeration member"})
	}
	return violations
}

// IndexSuffix is an owned encoded row suffix, not a complete object identifier.
type IndexSuffix struct{ arcs OID }

func (s IndexSuffix) Arcs() OID { return slices.Clone(s.arcs) }
func (s IndexSuffix) OID() OID  { return slices.Clone(s.arcs) }
func (s IndexSuffix) Len() int  { return len(s.arcs) }

// IndexEncodeErrorReason identifies canonical encoding failures.
type IndexEncodeErrorReason uint8

const (
	IndexEncodeInvalidLimit IndexEncodeErrorReason = iota + 1
	IndexEncodeWrongArity
	IndexEncodeWrongValueKind
	IndexEncodePrimitiveDomain
	IndexEncodeConstraintViolation
	IndexEncodeSuffixTooLong
)

// IndexEncodeError reports the failing component without returning partial output.
type IndexEncodeError struct {
	Reason   IndexEncodeErrorReason
	Position int
	Name     string
	Value    uint64
	Detail   string
}

func (e *IndexEncodeError) Error() string { return "index encode: " + e.Detail }

// EncodedIndex is a canonical suffix plus report-mode violations.
type EncodedIndex struct {
	suffix     IndexSuffix
	violations []IndexConstraintViolation
}

func (e *EncodedIndex) Suffix() IndexSuffix                    { return IndexSuffix{arcs: slices.Clone(e.suffix.arcs)} }
func (e *EncodedIndex) Violations() []IndexConstraintViolation { return slices.Clone(e.violations) }

// EncodeCanonical validates arity and semantic kinds, then encodes atomically.
func (s *IndexSchema) EncodeCanonical(values []IndexValue, options EncodeOptions) (*EncodedIndex, error) {
	if options.MaxSuffixArcs < 0 {
		return nil, &IndexEncodeError{Reason: IndexEncodeInvalidLimit, Position: -1, Detail: "negative suffix limit"}
	}
	if len(values) != len(s.components) {
		return nil, &IndexEncodeError{Reason: IndexEncodeWrongArity, Position: -1, Detail: fmt.Sprintf("got %d values for %d components", len(values), len(s.components))}
	}
	arcs := make(OID, 0, min(options.MaxSuffixArcs, max(s.minWidth, 0)))
	result := &EncodedIndex{}
	for i := range s.components {
		component := &s.components[i]
		length, numeric, err := validateWireValue(component.wireType, values[i])
		if err != nil {
			reason := IndexEncodeWrongValueKind
			if component.wireType.integerKind == IntegerIndexInteger32 && values[i].kind == IndexValueInteger32 {
				reason = IndexEncodePrimitiveDomain
			}
			return nil, &IndexEncodeError{Reason: reason, Position: i, Name: component.name, Detail: err.Error()}
		}
		constraintValue := numeric
		if component.wireType.kind != IndexWireInteger {
			constraintValue = uint64(length)
		}
		violations := componentViolations(i, component, constraintValue)
		if violation := firstKnownViolation(violations); violation != nil {
			return nil, &IndexEncodeError{Reason: IndexEncodeConstraintViolation, Position: i, Name: component.name, Value: constraintValue, Detail: violation.Detail}
		}
		if len(violations) > 0 && options.IncompleteConstraint == IncompleteConstraintReject {
			return nil, &IndexEncodeError{Reason: IndexEncodeConstraintViolation, Position: i, Name: component.name, Value: constraintValue, Detail: violations[0].Detail}
		}
		result.violations = append(result.violations, violations...)
		needed := length
		if component.wireType.framing == IndexFramingLengthPrefixed {
			needed++
		}
		if needed > options.MaxSuffixArcs-len(arcs) {
			return nil, &IndexEncodeError{Reason: IndexEncodeSuffixTooLong, Position: i, Name: component.name, Detail: "encoded suffix exceeds operation limit"}
		}
		if component.wireType.framing == IndexFramingLengthPrefixed {
			if uint64(length) > math.MaxUint32 {
				return nil, &IndexEncodeError{Reason: IndexEncodePrimitiveDomain, Position: i, Name: component.name, Detail: "length prefix exceeds uint32"}
			}
			arcs = append(arcs, uint32(length))
		}
		arcs = appendWireValue(arcs, component.wireType, values[i])
	}
	result.suffix = IndexSuffix{arcs: arcs}
	return result, nil
}

func validateWireValue(wire IndexWireType, value IndexValue) (int, uint64, error) {
	switch wire.kind {
	case IndexWireInteger:
		if wire.integerKind == IntegerIndexInteger32 {
			signed, ok := value.Signed()
			if !ok {
				return 0, 0, fmt.Errorf("expected Integer32 value")
			}
			if signed < 0 {
				return 0, 0, fmt.Errorf("negative Integer32 is not representable in an OID arc")
			}
			return 1, uint64(signed), nil
		}
		unsigned, ok := value.Unsigned()
		if !ok || !integerKindMatches(wire.integerKind, value.kind) {
			return 0, 0, fmt.Errorf("integer value kind does not match schema")
		}
		return 1, uint64(unsigned), nil
	case IndexWireIpAddress:
		if value.kind != IndexValueIpAddress {
			return 0, 0, fmt.Errorf("expected IpAddress value")
		}
		return 4, 4, nil
	case IndexWireOctets:
		if !octetKindMatches(wire.octetKind, value.kind) {
			return 0, 0, fmt.Errorf("octet value kind does not match schema")
		}
		length := len(value.value.([]byte))
		if wire.framing == IndexFramingFixed && length != wire.fixedWidth {
			return 0, 0, fmt.Errorf("value length does not match fixed width")
		}
		return length, uint64(length), nil
	case IndexWireObjectIdentifier:
		if value.kind != IndexValueObjectIdentifier {
			return 0, 0, fmt.Errorf("expected ObjectIdentifier value")
		}
		length := len(value.value.(OID))
		return length, uint64(length), nil
	}
	return 0, 0, fmt.Errorf("unsupported wire type")
}

func appendWireValue(arcs OID, wire IndexWireType, value IndexValue) OID {
	switch wire.kind {
	case IndexWireInteger:
		if wire.integerKind == IntegerIndexInteger32 {
			return append(arcs, uint32(value.value.(int32)))
		}
		return append(arcs, value.value.(uint32))
	case IndexWireIpAddress:
		ip := value.value.([4]byte)
		return append(arcs, uint32(ip[0]), uint32(ip[1]), uint32(ip[2]), uint32(ip[3]))
	case IndexWireOctets:
		for _, octet := range value.value.([]byte) {
			arcs = append(arcs, uint32(octet))
		}
		return arcs
	case IndexWireObjectIdentifier:
		return append(arcs, value.value.(OID)...)
	default:
		return arcs
	}
}

func integerKindMatches(kind IntegerIndexKind, value IndexValueKind) bool {
	return (kind == IntegerIndexUnsigned32 && value == IndexValueUnsigned32) ||
		(kind == IntegerIndexGauge32 && value == IndexValueGauge32) ||
		(kind == IntegerIndexTimeTicks && value == IndexValueTimeTicks) ||
		(kind == IntegerIndexCounter32 && value == IndexValueCounter32)
}

func octetKindMatches(kind OctetIndexKind, value IndexValueKind) bool {
	return (kind == OctetIndexOctetString && value == IndexValueOctetString) ||
		(kind == OctetIndexBits && value == IndexValueBits) ||
		(kind == OctetIndexOpaque && value == IndexValueOpaque)
}

func cloneIndexValue(value IndexValue) IndexValue {
	switch value.kind {
	case IndexValueOctetString, IndexValueBits, IndexValueOpaque:
		value.value = slices.Clone(value.value.([]byte))
	case IndexValueObjectIdentifier:
		value.value = slices.Clone(value.value.(OID))
	}
	return value
}
