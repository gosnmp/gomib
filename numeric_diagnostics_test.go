package gomib_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golangsnmp/gomib"
	"github.com/golangsnmp/gomib/internal/testutil"
	"github.com/golangsnmp/gomib/internal/types"
	"github.com/golangsnmp/gomib/mib"
)

func TestLoadIntegralOverflowDiagnostics(t *testing.T) {
	for _, strictness := range []mib.ResolverStrictness{mib.ResolverStrict, mib.ResolverNormal, mib.ResolverPermissive} {
		t.Run(strictness.String(), func(t *testing.T) {
			m := loadAtStrictness(t, "PROBLEM-INTEGRAL-OVERFLOW-MIB", strictness)
			counts := make(map[string]int)
			for _, diag := range m.Diagnostics() {
				if diag.Code == types.DiagInvalidI64 || diag.Code == types.DiagInvalidU32 {
					counts[diag.Code]++
					if diag.Module != "PROBLEM-INTEGRAL-OVERFLOW-MIB" || diag.Severity != mib.SeverityError {
						t.Errorf("unexpected overflow diagnostic: %+v", diag)
					}
				}
			}
			testutil.Equal(t, 6, counts[types.DiagInvalidI64], "signed conversion diagnostics")
			testutil.Equal(t, 10, counts[types.DiagInvalidU32], "OID/trap conversion diagnostics")
			valid := requireObject(t, m, "problemValidUnsignedDefault")
			if strictness == mib.ResolverStrict {
				// Counter64 is not imported by this conversion fixture.
				testutil.True(t, valid.DefaultValue().IsZero(), "unresolved type has no operational default")
			} else {
				testutil.Equal(t, "18446744073709551615", valid.DefaultValue().String(), "valid unsigned default")
			}
			after := m.Node("problemAfterOverflow")
			if after == nil {
				t.Fatal("missing definition following overflow")
			}
			testutil.Equal(t, "1.99998.42.7", after.OID().String(), "trailing valid OID")
		})
	}
}

func TestLoadNumericDiagnosticPolicy(t *testing.T) {
	const input = `NUMERIC-POLICY-MIB DEFINITIONS ::= BEGIN
Numbers ::= INTEGER { huge(9223372036854775808) }
Flags ::= BITS { negative(-1), large(128), excessive(524280) }
badOid OBJECT IDENTIFIER ::= { iso 4294967296 }
defaultRoot OBJECT IDENTIFIER ::= { iso 3 }
numericDefault OBJECT-TYPE
    SYNTAX OBJECT IDENTIFIER
    ACCESS read-only
    STATUS mandatory
    DESCRIPTION "Numeric OID default."
    DEFVAL { { defaultRoot named(6) 1 } }
    ::= { iso 99998 1 }
END
`
	codes := []string{types.DiagInvalidI64, types.DiagInvalidU32, types.DiagBitsNumberNegative, types.DiagBitsNumberLarge, types.DiagBitsNumberTooLarge}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			for _, ignore := range []bool{false, true} {
				cfg := mib.SilentConfig()
				cfg.FailAt = mib.SeveritySevere
				cfg.Overrides = map[string]mib.Severity{code: mib.SeveritySevere}
				if ignore {
					cfg.Ignore = []string{code}
				}
				src := &fakeSource{modules: map[string]fakeModule{"NUMERIC-POLICY-MIB": {content: []byte(input)}}}
				m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("NUMERIC-POLICY-MIB"), gomib.WithDiagnosticConfig(cfg))
				if m == nil {
					t.Fatalf("load returned no model: %v", err)
				}
				if ignore {
					if err != nil || len(m.Diagnostics()) != 0 {
						t.Fatalf("suppressed load = %v, diagnostics = %+v", err, m.Diagnostics())
					}
				} else {
					if !errors.Is(err, gomib.ErrDiagnosticThreshold) {
						t.Fatalf("load error = %v, want diagnostic threshold", err)
					}
					diags := m.Diagnostics()
					if len(diags) != 1 || diags[0].Code != code || diags[0].Severity != mib.SeveritySevere {
						t.Fatalf("rejection explanation = %+v", diags)
					}
				}
				valid := requireObject(t, m, "numericDefault")
				oid, ok := mib.DefValAs[mib.OID](valid.DefaultValue())
				if !ok {
					t.Fatalf("default = %+v, want OID", valid.DefaultValue())
				}
				testutil.Equal(t, "1.3.6.1", oid.String(), "valid named-root default remains operational")
			}
		})
	}
}

func TestLoadQualifiedFirstOidDefault(t *testing.T) {
	const input = `TEST-MIB DEFINITIONS ::= BEGIN
root OBJECT IDENTIFIER ::= { iso 3 }
obj OBJECT-TYPE
    SYNTAX OBJECT IDENTIFIER
    ACCESS read-only
    STATUS mandatory
    DESCRIPTION "Qualified root recovery."
    DEFVAL { { TEST-MIB.root 6 1 } }
    ::= { root 1 }
END
`
	src := &fakeSource{modules: map[string]fakeModule{"TEST-MIB": {content: []byte(input)}}}
	cfg := mib.VerboseConfig()
	cfg.FailAt = mib.SeverityFatal
	m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("TEST-MIB"), gomib.WithDiagnosticConfig(cfg))
	testutil.NoError(t, err, "load qualified-root recovery")
	obj := requireObject(t, m, "obj")
	value, ok := mib.DefValAs[mib.OID](obj.DefaultValue())
	if !ok || value.String() != "1.3.6.1" {
		t.Fatalf("qualified-first default = %+v", obj.DefaultValue())
	}
	var unresolved int
	for _, diag := range m.Diagnostics() {
		if diag.Code == types.DiagDefvalUnresolved {
			unresolved++
		}
	}
	testutil.Equal(t, 0, unresolved, "unresolved default diagnostic")
}
