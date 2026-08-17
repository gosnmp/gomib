package mib

import (
	"log/slog"

	"github.com/golangsnmp/gomib/internal/model"
	"github.com/golangsnmp/gomib/internal/module"
	"github.com/golangsnmp/gomib/internal/resolver"
)

// --- Core model types ---

type (
	Mib          = model.Mib
	Node         = model.Node
	Module       = model.Module
	Object       = model.Object
	Type         = model.Type
	Notification = model.Notification
	Group        = model.Group
	Compliance   = model.Compliance
	Capability   = model.Capability
	Symbol       = model.Symbol
	SymbolKind   = model.SymbolKind
	OidLookup    = model.OidLookup
)

// --- OID ---

type OID = model.OID

func ParseOID(s string) (OID, error) { return model.ParseOID(s) }

// --- Value types ---

type (
	Range             = model.Range
	RangeBound        = model.RangeBound
	RangeBoundKind    = model.RangeBoundKind
	NamedValue        = model.NamedValue
	Revision          = model.Revision
	IndexEntry        = model.IndexEntry
	Import            = model.Import
	ImportSymbol      = model.ImportSymbol
	OidRef            = model.OidRef
	TrapInfo          = model.TrapInfo
	SyntaxConstraints = model.SyntaxConstraints
)

const (
	RangeBoundSigned   = model.RangeBoundSigned
	RangeBoundUnsigned = model.RangeBoundUnsigned
	RangeBoundMin      = model.RangeBoundMin
	RangeBoundMax      = model.RangeBoundMax
	RangeBoundRaw      = model.RangeBoundRaw
)

func NewSignedRangeBound(value int64) RangeBound    { return model.NewSignedRangeBound(value) }
func NewUnsignedRangeBound(value uint64) RangeBound { return model.NewUnsignedRangeBound(value) }
func NewRawRangeBound(value string) RangeBound      { return model.NewRawRangeBound(value) }

// --- DefVal ---

type (
	DefValKind = model.DefValKind
	DefVal     = model.DefVal
)

const (
	SymbolKindNone              = model.SymbolKindNone
	SymbolKindObject            = model.SymbolKindObject
	SymbolKindType              = model.SymbolKindType
	SymbolKindTextualConvention = model.SymbolKindTextualConvention
	SymbolKindNotification      = model.SymbolKindNotification
	SymbolKindGroup             = model.SymbolKindGroup
	SymbolKindNotificationGroup = model.SymbolKindNotificationGroup
	SymbolKindCompliance        = model.SymbolKindCompliance
	SymbolKindCapability        = model.SymbolKindCapability
	SymbolKindNode              = model.SymbolKindNode
)

const (
	DefValKindUnset  = model.DefValKindUnset
	DefValKindInt    = model.DefValKindInt
	DefValKindUint   = model.DefValKindUint
	DefValKindString = model.DefValKindString
	DefValKindBytes  = model.DefValKindBytes
	DefValKindEnum   = model.DefValKindEnum
	DefValKindBits   = model.DefValKindBits
	DefValKindOID    = model.DefValKindOID
)

// DefValAs extracts the default value as the concrete type T.
func DefValAs[T any](d DefVal) (T, bool) { return model.DefValAs[T](d) }

// --- Unresolved references ---

type (
	UnresolvedKind = model.UnresolvedKind
	UnresolvedRef  = model.UnresolvedRef
)

const (
	UnresolvedImport             = model.UnresolvedImport
	UnresolvedType               = model.UnresolvedType
	UnresolvedOID                = model.UnresolvedOID
	UnresolvedIndex              = model.UnresolvedIndex
	UnresolvedNotificationObject = model.UnresolvedNotificationObject
)

// --- Display hint ---

type (
	DisplayHint     = model.DisplayHint
	IntegerHint     = model.IntegerHint
	IntegerFormat   = model.IntegerFormat
	OctetStringHint = model.OctetStringHint
	OctetSegment    = model.OctetSegment
	OctetFormat     = model.OctetFormat
	HexCase         = model.HexCase
)

const (
	HexUpper = model.HexUpper
	HexLower = model.HexLower
)

const (
	IntegerDecimal = model.IntegerDecimal
	IntegerHex     = model.IntegerHex
	IntegerOctal   = model.IntegerOctal
	IntegerBinary  = model.IntegerBinary
)

const (
	OctetDecimal = model.OctetDecimal
	OctetHex     = model.OctetHex
	OctetOctal   = model.OctetOctal
	OctetAscii   = model.OctetAscii
	OctetUtf8    = model.OctetUtf8
)

func ParseDisplayHint(hint string) *DisplayHint { return model.ParseDisplayHint(hint) }
func IsValidIntegerHint(hint string) bool       { return model.IsValidIntegerHint(hint) }
func IsValidOctetStringHint(hint string) bool   { return model.IsValidOctetStringHint(hint) }
func FormatInteger(hint string, value int64, hexCase HexCase) (string, bool) {
	return model.FormatInteger(hint, value, hexCase)
}
func ScaleInteger(hint string, value int64) (float64, bool) { return model.ScaleInteger(hint, value) }

// FormatOctets formats an octet string according to an RFC 2579 display hint.
// Hexadecimal (x) segments produce exactly two digits per byte, including
// leading zeroes, and support segment lengths above eight bytes.
func FormatOctets(hint string, data []byte, hexCase HexCase) (string, bool) {
	return model.FormatOctets(hint, data, hexCase)
}

// --- Index schemas and codecs ---

type (
	IndexValueKind           = model.IndexValueKind
	IndexValue               = model.IndexValue
	ConstraintState          = model.ConstraintState
	ConstraintResult         = model.ConstraintResult
	ConstraintMode           = model.ConstraintMode
	IncompleteConstraintMode = model.IncompleteConstraintMode
	IndexRange               = model.IndexRange
	IncompleteIndexRange     = model.IncompleteIndexRange
	IndexConstraint          = model.IndexConstraint
	IndexFraming             = model.IndexFraming
	IndexWireKind            = model.IndexWireKind
	IntegerIndexKind         = model.IntegerIndexKind
	OctetIndexKind           = model.OctetIndexKind
	IndexWireType            = model.IndexWireType
	IndexSchemaIssueCode     = model.IndexSchemaIssueCode
	IndexSchemaIssue         = model.IndexSchemaIssue
	IndexComponentSchema     = model.IndexComponentSchema
	IndexSchema              = model.IndexSchema
	BoundIndexCodec          = model.BoundIndexCodec
	IndexSchemaErrorReason   = model.IndexSchemaErrorReason
	IndexSchemaError         = model.IndexSchemaError
	DecodeOptions            = model.DecodeOptions
	EncodeOptions            = model.EncodeOptions
	IndexConstraintViolation = model.IndexConstraintViolation
	DecodedIndexComponent    = model.DecodedIndexComponent
	DecodedRowIndex          = model.DecodedRowIndex
	IndexDecodeErrorReason   = model.IndexDecodeErrorReason
	IndexDecodeError         = model.IndexDecodeError
	IndexSuffix              = model.IndexSuffix
	IndexEncodeErrorReason   = model.IndexEncodeErrorReason
	IndexEncodeError         = model.IndexEncodeError
	EncodedIndex             = model.EncodedIndex
)

// MaxInstanceOIDArcs is the SMI limit for a complete instance identifier.
const MaxInstanceOIDArcs = model.MaxInstanceOIDArcs

const (
	IndexValueIpAddress        = model.IndexValueIpAddress
	IndexValueOctetString      = model.IndexValueOctetString
	IndexValueObjectIdentifier = model.IndexValueObjectIdentifier
	IndexValueInteger32        = model.IndexValueInteger32
	IndexValueUnsigned32       = model.IndexValueUnsigned32
	IndexValueGauge32          = model.IndexValueGauge32
	IndexValueTimeTicks        = model.IndexValueTimeTicks
	IndexValueCounter32        = model.IndexValueCounter32
	IndexValueBits             = model.IndexValueBits
	IndexValueOpaque           = model.IndexValueOpaque
)

const (
	ConstraintUnspecified      = model.ConstraintUnspecified
	ConstraintKnown            = model.ConstraintKnown
	ConstraintIncomplete       = model.ConstraintIncomplete
	ConstraintEmpty            = model.ConstraintEmpty
	ConstraintValid            = model.ConstraintValid
	ConstraintInvalid          = model.ConstraintInvalid
	ConstraintIndeterminate    = model.ConstraintIndeterminate
	ConstraintEnforce          = model.ConstraintEnforce
	ConstraintReport           = model.ConstraintReport
	IncompleteConstraintAllow  = model.IncompleteConstraintAllow
	IncompleteConstraintReject = model.IncompleteConstraintReject
)

const (
	IndexFramingNone           = model.IndexFramingNone
	IndexFramingFixed          = model.IndexFramingFixed
	IndexFramingLengthPrefixed = model.IndexFramingLengthPrefixed
	IndexFramingImplied        = model.IndexFramingImplied
	IndexWireInteger           = model.IndexWireInteger
	IndexWireIpAddress         = model.IndexWireIpAddress
	IndexWireOctets            = model.IndexWireOctets
	IndexWireObjectIdentifier  = model.IndexWireObjectIdentifier
	IntegerIndexInteger32      = model.IntegerIndexInteger32
	IntegerIndexUnsigned32     = model.IntegerIndexUnsigned32
	IntegerIndexGauge32        = model.IntegerIndexGauge32
	IntegerIndexTimeTicks      = model.IntegerIndexTimeTicks
	IntegerIndexCounter32      = model.IntegerIndexCounter32
	OctetIndexOctetString      = model.OctetIndexOctetString
	OctetIndexBits             = model.OctetIndexBits
	OctetIndexOpaque           = model.OctetIndexOpaque
)

const (
	IndexSchemaCounter32Extension        = model.IndexSchemaCounter32Extension
	IndexSchemaUnrepresentableConstraint = model.IndexSchemaUnrepresentableConstraint
	IndexSchemaUnrepresentableEnum       = model.IndexSchemaUnrepresentableEnum
	IndexSchemaImpliedOctetsMayBeEmpty   = model.IndexSchemaImpliedOctetsMayBeEmpty
	IndexSchemaUnresolvedObjectIdentity  = model.IndexSchemaUnresolvedObjectIdentity
	IndexSchemaZeroWidthComponent        = model.IndexSchemaZeroWidthComponent
	IndexSchemaZeroWidthIndex            = model.IndexSchemaZeroWidthIndex
	IndexSchemaNoIndexes                 = model.IndexSchemaNoIndexes
	IndexSchemaInvalidObject             = model.IndexSchemaInvalidObject
	IndexSchemaUnresolvedType            = model.IndexSchemaUnresolvedType
	IndexSchemaUnsupportedType           = model.IndexSchemaUnsupportedType
	IndexSchemaInvalidImplied            = model.IndexSchemaInvalidImplied
	IndexSchemaEmptyConstraint           = model.IndexSchemaEmptyConstraint
	IndexSchemaOverflow                  = model.IndexSchemaOverflow
)

const (
	IndexDecodeInvalidLimit        = model.IndexDecodeInvalidLimit
	IndexDecodeSuffixTooLong       = model.IndexDecodeSuffixTooLong
	IndexDecodeTruncated           = model.IndexDecodeTruncated
	IndexDecodeLengthExceedsLimit  = model.IndexDecodeLengthExceedsLimit
	IndexDecodeInvalidOctet        = model.IndexDecodeInvalidOctet
	IndexDecodePrimitiveDomain     = model.IndexDecodePrimitiveDomain
	IndexDecodeTrailingArcs        = model.IndexDecodeTrailingArcs
	IndexDecodeConstraintViolation = model.IndexDecodeConstraintViolation
	IndexEncodeInvalidLimit        = model.IndexEncodeInvalidLimit
	IndexEncodeWrongArity          = model.IndexEncodeWrongArity
	IndexEncodeWrongValueKind      = model.IndexEncodeWrongValueKind
	IndexEncodePrimitiveDomain     = model.IndexEncodePrimitiveDomain
	IndexEncodeConstraintViolation = model.IndexEncodeConstraintViolation
	IndexEncodeSuffixTooLong       = model.IndexEncodeSuffixTooLong
)

// CompileIndexSchema copies and normalizes a row or column's effective INDEX
// metadata into an owned schema.
func CompileIndexSchema(object *Object) (*IndexSchema, error) {
	return model.CompileIndexSchema(object)
}

// NewBoundIndexCodec binds an owned schema to a destination OID and explicit
// complete instance-OID limit.
func NewBoundIndexCodec(schema *IndexSchema, objectOID OID, maxInstanceArcs int) (*BoundIndexCodec, error) {
	return model.NewBoundIndexCodec(schema, objectOID, maxInstanceArcs)
}

// BindIndexSchema binds an owned schema to an object's OID using the SMI
// 128-arc complete instance-identifier limit.
func BindIndexSchema(schema *IndexSchema, object *Object) (*BoundIndexCodec, error) {
	return model.BindIndexSchema(schema, object)
}

// NewInteger32IndexValue returns a signed Integer32 index value.
func NewInteger32IndexValue(value int32) IndexValue { return model.NewInteger32IndexValue(value) }

// NewUnsigned32IndexValue returns an Unsigned32 index value.
func NewUnsigned32IndexValue(value uint32) IndexValue { return model.NewUnsigned32IndexValue(value) }

// NewGauge32IndexValue returns a Gauge32 index value.
func NewGauge32IndexValue(value uint32) IndexValue { return model.NewGauge32IndexValue(value) }

// NewTimeTicksIndexValue returns a TimeTicks index value.
func NewTimeTicksIndexValue(value uint32) IndexValue { return model.NewTimeTicksIndexValue(value) }

// NewCounter32IndexValue returns a mechanically representable Counter32 index value.
func NewCounter32IndexValue(value uint32) IndexValue { return model.NewCounter32IndexValue(value) }

// NewIpAddressIndexValue returns an IPv4 index value.
func NewIpAddressIndexValue(value [4]byte) IndexValue { return model.NewIpAddressIndexValue(value) }

// NewOctetStringIndexValue returns an owned OCTET STRING index value.
func NewOctetStringIndexValue(value []byte) IndexValue { return model.NewOctetStringIndexValue(value) }

// NewBitsIndexValue returns an owned BITS index value.
func NewBitsIndexValue(value []byte) IndexValue { return model.NewBitsIndexValue(value) }

// NewOpaqueIndexValue returns an owned Opaque index value.
func NewOpaqueIndexValue(value []byte) IndexValue { return model.NewOpaqueIndexValue(value) }

// NewObjectIdentifierIndexValue returns an owned OBJECT IDENTIFIER index value.
func NewObjectIdentifierIndexValue(value OID) IndexValue {
	return model.NewObjectIdentifierIndexValue(value)
}

// NameRefNames returns just the name strings from a slice of NameRefs.
var NameRefNames = model.NameRefNames

// --- Compliance / Capability value types ---

type (
	ComplianceModule      = model.ComplianceModule
	ComplianceGroup       = model.ComplianceGroup
	ComplianceObject      = model.ComplianceObject
	CapabilitiesModule    = model.CapabilitiesModule
	ObjectVariation       = model.ObjectVariation
	NotificationVariation = model.NotificationVariation
	NameRef               = model.NameRef
)

// --- Span context ---

type (
	SpanContext     = model.SpanContext
	SpanContextKind = model.SpanContextKind
)

const (
	SpanContextNone       = model.SpanContextNone
	SpanContextDefinition = model.SpanContextDefinition
	SpanContextImport     = model.SpanContextImport
	SpanContextOidRef     = model.SpanContextOidRef
	SpanContextSyntax     = model.SpanContextSyntax
)

// --- Resolve entry point ---

// Resolve transforms normalized modules into a fully resolved [Mib].
//
// Resolution runs these phases in order:
//
//  1. Registration: index modules and definitions
//  2. Imports: resolve imported symbols across modules
//  3. Types: build type links and compute base types
//  4. OIDs: build the OID tree from symbolic references
//  5. Semantics: infer node kinds and create objects
//
// If logger is nil, logging is disabled. If strictness is nil, defaults to
// [ResolverNormal]. If diagConfig is nil, defaults to [DefaultConfig].
func Resolve(mods []*module.Module, logger *slog.Logger, strictness *ResolverStrictness, diagConfig *DiagnosticConfig) *Mib {
	return resolver.Resolve(mods, logger, strictness, diagConfig)
}
