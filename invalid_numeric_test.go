package gomib_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/golangsnmp/gomib"
	"github.com/golangsnmp/gomib/internal/testutil"
	"github.com/golangsnmp/gomib/internal/types"
	"github.com/golangsnmp/gomib/mib"
	"github.com/golangsnmp/gomib/syntax"
)

func TestInvalidNumericMetadata(t *testing.T) {
	content, err := os.ReadFile("testdata/fixtures/invalid-numeric.mib")
	testutil.NoError(t, err, "read required fixture")
	for _, strictness := range []mib.ResolverStrictness{mib.ResolverStrict, mib.ResolverNormal, mib.ResolverPermissive} {
		for _, suppress := range []bool{false, true} {
			t.Run(strictness.String()+map[bool]string{false: "/reported", true: "/suppressed"}[suppress], func(t *testing.T) {
				cfg := mib.VerboseConfig()
				cfg.FailAt = mib.SeverityFatal
				if suppress {
					cfg = mib.SilentConfig()
					cfg.FailAt = mib.SeverityFatal
					cfg.Ignore = []string{types.DiagInvalidI64, types.DiagInvalidU32, types.DiagBitsNumberNegative, types.DiagBitsNumberTooLarge, types.DiagUnknownRangeValue}
				}
				src := &fakeSource{modules: map[string]fakeModule{"INVALID-NUMERIC-MIB": {content: content}}}
				m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("INVALID-NUMERIC-MIB"), gomib.WithResolverStrictness(strictness), gomib.WithDiagnosticConfig(cfg))
				testutil.NoError(t, err, "load malformed metadata")
				for _, name := range []string{"badPlain", "badNamed", "badKnown", "badQualified", "negativePlain", "negativeNamed", "negativeQualified", "negativeFirst", "dependent", "phantom", "invalid", "badTrap"} {
					if n := m.Node(name); n != nil {
						t.Errorf("invalid name %s resolved as %v", name, n.OID())
					}
				}
				for _, name := range []string{"BadEnum", "BadEnumAlias", "BadBits", "BadBitsAlias", "BadLargeBits", "BadEnumRestricted", "BadBitsRefinement", "BadBitsRefinementAlias"} {
					typ := m.Type(name)
					if typ == nil || typ.EffectiveBase() != mib.BaseUnknown || len(typ.EffectiveEnums()) != 0 || len(typ.EffectiveBits()) != 0 {
						t.Errorf("%s must retain unresolved type identity without partial values: %+v", name, typ)
					}
				}
				for _, name := range []string{"badEnumObject", "badBitsObject", "badInlineObject", "badPositiveDefault", "badNegativeDefault", "badOidDefault", "badKnownDefault", "badQualifiedDefault", "badBinaryDefault", "badReferenceDefault", "badRestrictedObject", "badRefinedBitsObject", "badInlineBitsObject", "negativeTailDefault", "negativeSmallDefault", "negativeNamedDefault", "negativeQualifiedDefault", "negativeFirstDefault", "negativeNamedRootDefault", "negativeQualifiedRootDefault"} {
					obj := requireObject(t, m, name)
					if !obj.DefaultValue().IsZero() {
						t.Errorf("%s published invalid default %+v", name, obj.DefaultValue())
					}
					if obj.Node() == nil || obj.DefaultValueSpan().Start == obj.DefaultValueSpan().End {
						t.Errorf("%s lost valid OID or source default location", name)
					}
				}
				for name, want := range map[string]string{"validDefault": "0", "validSignedDefault": "-9223372036854775808", "validUnsignedDefault": "18446744073709551615"} {
					testutil.Equal(t, want, requireObject(t, m, name).DefaultValue().String(), name)
				}
				for name, want := range map[string]string{"validZero": "1.3.6.1.4.1.99997.0", "validMaximum": "1.3.6.1.4.1.99997.4294967295", "after": "1.3.6.1.4.1.99997.30", "validTrapZero": "1.3.6.1.4.1.99997.0.0", "validTrapMaximum": "1.3.6.1.4.1.99997.0.4294967295"} {
					n := m.Node(name)
					if n == nil || n.OID().String() != want {
						t.Errorf("%s OID = %v, want %s", name, n, want)
					}
				}
				validBits := m.Type("ValidBits").EffectiveBits()
				if len(validBits) != 3 || validBits[0].Value != 0 || validBits[1].Value != 128 || validBits[2].Value != 524279 {
					t.Fatalf("valid BITS boundaries = %+v", validBits)
				}
				_, err = mib.CompileIndexSchema(requireObject(t, m, "badValue"))
				var schemaErr *mib.IndexSchemaError
				if !errors.As(err, &schemaErr) || schemaErr.Reason != mib.IndexSchemaUnresolvedType {
					t.Errorf("invalid enum index schema = %v", err)
				}
				schema, err := mib.CompileIndexSchema(requireObject(t, m, "rangeValue"))
				testutil.NoError(t, err, "compile incomplete constraint schema")
				constraint := schema.Component(0).WireType().Constraint()
				if constraint.State() != mib.ConstraintIncomplete {
					t.Fatalf("overflow constraint state = %v", constraint.State())
				}
				_, err = schema.EncodeCanonical([]mib.IndexValue{mib.NewOctetStringIndexValue(nil)}, mib.EncodeOptions{MaxSuffixArcs: 8, IncompleteConstraint: mib.IncompleteConstraintReject})
				if err == nil {
					t.Fatal("incomplete length constraint accepted as valid zero")
				}
			})
		}
	}
	file, _ := syntax.Parse(content)
	testutil.Equal(t, string(content), string(syntax.ReconstructText(file, content)), "lossless malformed source")
}

func TestInvalidNumericTypeFallbacks(t *testing.T) {
	for _, tc := range []struct {
		module, source string
		names          []string
	}{
		{"INVALID-PRIMITIVE-MIB", `INVALID-PRIMITIVE-MIB DEFINITIONS ::= BEGIN
Integer32 ::= INTEGER { bad(9223372036854775808), good(1) }
Alias ::= Integer32
Restricted ::= Alias { good(1) }
END`, []string{"Integer32", "Alias", "Restricted"}},
		{"SNMPv2-SMI", `SNMPv2-SMI DEFINITIONS ::= BEGIN
Integer32 ::= INTEGER { bad(9223372036854775808), good(1) }
Counter64 ::= Integer32
END`, []string{"Integer32", "Counter64"}},
		{"RFC1213-MIB", `RFC1213-MIB DEFINITIONS ::= BEGIN
DisplayString ::= INTEGER { bad(9223372036854775808), good(1) }
PhysAddress ::= BITS { bad(-1), good(1) }
END`, []string{"DisplayString", "PhysAddress"}},
	} {
		t.Run(tc.module, func(t *testing.T) {
			src := &fakeSource{modules: map[string]fakeModule{tc.module: {content: []byte(tc.source)}}}
			cfg := mib.VerboseConfig()
			cfg.FailAt = mib.SeverityFatal
			m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules(tc.module, "SNMPv2-TC"), gomib.WithDiagnosticConfig(cfg))
			testutil.NoError(t, err, "load invalid named types")
			mod := m.Module(tc.module)
			for _, name := range tc.names {
				typ := mod.Type(name)
				if typ == nil || typ.EffectiveBase() != mib.BaseUnknown || len(typ.EffectiveEnums()) != 0 || len(typ.EffectiveBits()) != 0 {
					t.Errorf("fallback repaired invalid %s: %v", name, typ)
				}
			}
		})
	}
}

func TestInvalidIndexDefaultStillDiagnosed(t *testing.T) {
	content, err := os.ReadFile("testdata/fixtures/invalid-numeric.mib")
	testutil.NoError(t, err, "read required fixture")
	input := strings.Replace(string(content), `DESCRIPTION "Invalid index type." ::= { badEntry 1 }`, `DESCRIPTION "Invalid index type." DEFVAL { 0 } ::= { badEntry 1 }`, 1)
	if input == string(content) {
		t.Fatal("fixture index definition missing")
	}
	src := &fakeSource{modules: map[string]fakeModule{"INVALID-NUMERIC-MIB": {content: []byte(input)}}}
	cfg := mib.VerboseConfig()
	cfg.FailAt = mib.SeverityFatal
	m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("INVALID-NUMERIC-MIB"), gomib.WithDiagnosticConfig(cfg))
	testutil.NoError(t, err, "load invalid index default")
	testutil.True(t, requireObject(t, m, "badIndex").DefaultValue().IsZero(), "unresolved index default")
	count := 0
	for _, diagnostic := range m.Diagnostics() {
		if diagnostic.Code == types.DiagIndexDefval {
			count++
		}
	}
	testutil.Equal(t, 1, count, "written INDEX default remains diagnosed")
}
