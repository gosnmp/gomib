package lower_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/golangsnmp/gomib/internal/cst"
	cstparser "github.com/golangsnmp/gomib/internal/cst/parser"
	"github.com/golangsnmp/gomib/internal/module"
	"github.com/golangsnmp/gomib/internal/testutil"
	"github.com/golangsnmp/gomib/internal/types"
)

func TestIntegralOverflowDiagnostics(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(testutil.ProblemsCorpusDir(t), "PROBLEM-INTEGRAL-OVERFLOW-MIB.mib"))
	testutil.NoError(t, err, "read overflow fixture")
	mods := parseMods(t, string(source))
	if len(mods) != 1 {
		t.Fatalf("modules = %d, want 1", len(mods))
	}
	var numeric []types.Diagnostic
	for _, diag := range mods[0].Diagnostics {
		if diag.Code == types.DiagInvalidI64 || diag.Code == types.DiagInvalidU32 {
			numeric = append(numeric, diag)
		}
	}
	want := []struct {
		code         string
		line, column int
		token        string
	}{
		{types.DiagInvalidI64, 11, 10, "9223372036854775808"},
		{types.DiagInvalidI64, 12, 9, "-9223372036854775809"},
		{types.DiagInvalidI64, 17, 10, "9223372036854775809"},
		{types.DiagInvalidI64, 18, 9, "-9223372036854775810"},
		{types.DiagInvalidU32, 23, 33, "4294967296"},
		{types.DiagInvalidU32, 27, 42, "4294967297"},
		{types.DiagInvalidU32, 31, 63, "4294967298"},
		{types.DiagInvalidI64, 39, 19, "18446744073709551616"},
		{types.DiagInvalidI64, 55, 19, "-9223372036854775811"},
		{types.DiagInvalidU32, 64, 15, "4294967299"},
		{types.DiagInvalidU32, 65, 9, "4294967300"},
		{types.DiagInvalidU32, 66, 67, "4294967301"},
		{types.DiagInvalidU32, 67, 15, "4294967302"},
		{types.DiagInvalidU32, 76, 21, "4294967304"},
		{types.DiagInvalidU32, 76, 38, "4294967305"},
		{types.DiagInvalidU32, 82, 9, "4294967303"},
	}
	if len(numeric) != len(want) {
		t.Fatalf("numeric diagnostics = %+v, want exactly %d", numeric, len(want))
	}
	lines := strings.Split(string(source), "\n")
	for i, expected := range want {
		got := numeric[i]
		if got.Code != expected.code || got.Module != "PROBLEM-INTEGRAL-OVERFLOW-MIB" ||
			got.Severity != types.SeverityError || got.Line != expected.line || got.Column != expected.column ||
			got.EndLine != expected.line || got.EndColumn != expected.column+len(expected.token) {
			t.Errorf("diagnostic %d = %+v, want %+v at error severity", i, got, expected)
		}
		if !strings.Contains(got.Message, expected.token) || !strings.Contains(got.Message, "not a valid") {
			t.Errorf("diagnostic %d lacks conversion explanation: %q", i, got.Message)
		}
		if got.Line > 0 && got.Line <= len(lines) && got.Column > 0 && got.EndColumn <= len(lines[got.Line-1])+1 {
			testutil.Equal(t, expected.token, lines[got.Line-1][got.Column-1:got.EndColumn-1], "diagnostic source token")
		}
	}

	// Lowering and diagnostics must leave malformed syntax available for inspection.
	parser := cstparser.New(source, nil, types.VerboseConfig())
	file := parser.ParseModule()
	testutil.Equal(t, string(source), string(cst.ReconstructText(file, source)), "lossless overflow source")
	last := mods[0].ValueAssignments[len(mods[0].ValueAssignments)-1]
	testutil.Equal(t, "problemAfterOverflow", last.Name, "definition after overflows")
	unsigned := mods[0].ObjectTypes[1].DefVal.(*module.DefValUnsigned)
	testutil.Equal(t, uint64(18446744073709551615), unsigned.Value, "valid unsigned default")
}

func TestNegativeOidDefaultComponents(t *testing.T) {
	for _, expression := range []string{"root -1 2", "root -4294967296 2", "root(-1) 2", "TEST-MIB.root(-1) 2", "-1 2"} {
		t.Run(expression, func(t *testing.T) {
			source := `TEST-MIB DEFINITIONS ::= BEGIN
bad OBJECT-TYPE SYNTAX OBJECT IDENTIFIER ACCESS read-only STATUS mandatory
DESCRIPTION "Bad." DEFVAL { { ` + expression + ` } } ::= { iso 3 }
END`
			mod := parseMods(t, source)[0]
			if len(mod.ObjectTypes) != 1 {
				t.Fatalf("object lost: %+v", mod.ObjectTypes)
			}
			dv, ok := mod.ObjectTypes[0].DefVal.(*module.DefValOidValue)
			if !ok {
				t.Fatalf("default = %T", mod.ObjectTypes[0].DefVal)
			}
			invalid := 0
			for _, comp := range dv.Components {
				if comp.Invalid {
					invalid++
					if comp.HasNumber {
						t.Fatal("invalid component has operational number")
					}
				}
			}
			if invalid != 1 {
				t.Fatalf("invalid components = %d: %+v", invalid, dv.Components)
			}
			numeric := 0
			for _, diag := range mod.Diagnostics {
				if diag.Code == types.DiagInvalidU32 {
					numeric++
					line := strings.Split(source, "\n")[diag.Line-1]
					token := line[diag.Column-1 : diag.EndColumn-1]
					if token != "-1" && token != "-4294967296" {
						t.Fatalf("invalid-u32 source = %q", token)
					}
				}
			}
			if numeric != 1 {
				t.Fatalf("numeric diagnostics = %d", numeric)
			}
		})
	}
}

func TestBitsPositionBoundaries(t *testing.T) {
	const source = `BITS-BOUNDARY-MIB DEFINITIONS ::= BEGIN
Flags ::= BITS {
    negative(-1),
    zero(0),
    small(127),
    large(128),
    maximum(524279),
    excessive(524280),
    otherNegative(-2),
    otherExcessive(524281),
    duplicateNegative(-1),
    duplicateExcessive(524280)
}
END
`
	mods := parseMods(t, source)
	if _, invalid := mods[0].TypeDefs[0].Syntax.(*module.TypeSyntaxInvalid); !invalid {
		t.Fatalf("invalid BITS expression became operational: %#v", mods[0].TypeDefs[0].Syntax)
	}
	want := map[string][]int{
		types.DiagBitsNumberNegative:    {3, 9, 11},
		types.DiagBitsNumberLarge:       {6, 7},
		types.DiagBitsNumberTooLarge:    {8, 10, 12},
		types.DiagBitsValueRedefinition: {11, 12},
	}
	lines := strings.Split(source, "\n")
	for code, expectedLines := range want {
		var gotLines []int
		for _, diag := range mods[0].Diagnostics {
			if diag.Code != code {
				continue
			}
			gotLines = append(gotLines, diag.Line)
			severity := types.SeverityError
			if code == types.DiagBitsNumberLarge {
				severity = types.SeverityStyle
			}
			if diag.Module != "BITS-BOUNDARY-MIB" || diag.Severity != severity || diag.Column != 5 || diag.EndLine != diag.Line || diag.EndColumn != len(strings.TrimSuffix(lines[diag.Line-1], ","))+1 {
				t.Errorf("unexpected BITS diagnostic: %+v", diag)
			}
		}
		if !slices.Equal(gotLines, expectedLines) {
			t.Errorf("%s lines = %v, want %v", code, gotLines, expectedLines)
		}
	}
}

func TestNumericDiagnosticPolicy(t *testing.T) {
	const source = `POLICY-MIB DEFINITIONS ::= BEGIN
Numbers ::= INTEGER { huge(9223372036854775808) }
Flags ::= BITS { negative(-1), large(128), excessive(524280) }
root OBJECT IDENTIFIER ::= { iso 4294967296 }
END
`
	codes := []string{types.DiagInvalidI64, types.DiagInvalidU32, types.DiagBitsNumberNegative, types.DiagBitsNumberLarge, types.DiagBitsNumberTooLarge}
	for _, ignored := range []bool{false, true} {
		t.Run(fmt.Sprintf("ignored=%t", ignored), func(t *testing.T) {
			cfg := types.VerboseConfig()
			cfg.Overrides = make(map[string]types.Severity)
			for _, code := range codes {
				cfg.Overrides[code] = types.SeverityWarning
			}
			if ignored {
				cfg.Ignore = codes
			}
			mods := parseModsWithConfig(t, source, cfg)
			for _, code := range codes {
				diag := findDiag(mods, code)
				if ignored {
					if diag != nil {
						t.Errorf("ignored %s retained", code)
					}
				} else if diag == nil || diag.Severity != types.SeverityWarning {
					t.Errorf("%s override diagnostic = %+v", code, diag)
				}
			}
			for _, definition := range mods[0].TypeDefs {
				if _, invalid := definition.Syntax.(*module.TypeSyntaxInvalid); !invalid {
					t.Errorf("%s numeric recovery became operational", definition.Name)
				}
			}
			if !mods[0].ValueAssignments[0].Oid.Components[1].Invalid {
				t.Fatal("OID overflow lost invalid state")
			}
		})
	}
}

func TestNumericFirstOidDefault(t *testing.T) {
	const source = `DEFAULT-MIB DEFINITIONS ::= BEGIN
numeric OBJECT-TYPE
    SYNTAX OBJECT IDENTIFIER
    ACCESS read-only
    STATUS mandatory
    DESCRIPTION "Numeric OID default."
    DEFVAL { { 1 3 named(6) 1 } }
    ::= { iso 99998 1 }
END
`
	mods := parseMods(t, source)
	components := mods[0].ObjectTypes[0].DefVal.(*module.DefValOidValue).Components
	if len(components) != 4 {
		t.Fatalf("OID components = %+v", components)
	}
	for i, want := range []uint32{1, 3, 6, 1} {
		if !components[i].HasNumber || components[i].Number != want {
			t.Errorf("component %d = %+v", i, components[i])
		}
	}
	testutil.Equal(t, "", components[0].Name, "numeric first arc has no name")
}

func TestKeywordFirstOidDefault(t *testing.T) {
	const source = `DEFAULT-MIB DEFINITIONS ::= BEGIN
keyword OBJECT-TYPE
    SYNTAX OBJECT IDENTIFIER
    ACCESS read-only
    STATUS mandatory
    DESCRIPTION "Keyword root label."
    DEFVAL { { INTEGER(1) 3 } }
    ::= { iso 99998 1 }
END
`
	mods := parseMods(t, source)
	components := mods[0].ObjectTypes[0].DefVal.(*module.DefValOidValue).Components
	if len(components) != 2 {
		t.Fatalf("OID components = %+v", components)
	}
	testutil.Equal(t, "INTEGER", components[0].Name, "keyword root retained")
	for i, want := range []uint32{1, 3} {
		if !components[i].HasNumber || components[i].Number != want {
			t.Errorf("component %d = %+v", i, components[i])
		}
	}
}

func TestOidDefaultComponentForms(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    []module.OidComponent
	}{
		{"named root", "root 6 1", []module.OidComponent{{Name: "root"}, {Number: 6, HasNumber: true}, {Number: 1, HasNumber: true}}},
		{"numbered root", "root(3) 6 1", []module.OidComponent{{Name: "root", Number: 3, HasNumber: true}, {Number: 6, HasNumber: true}, {Number: 1, HasNumber: true}}},
		{"qualified tail", "root TEST-MIB.child(6) 1", []module.OidComponent{{Name: "root"}, {Module: "TEST-MIB", Name: "child", Number: 6, HasNumber: true}, {Number: 1, HasNumber: true}}},
		{"qualified root", "TEST-MIB.root 6 1", []module.OidComponent{{Module: "TEST-MIB", Name: "root"}, {Number: 6, HasNumber: true}, {Number: 1, HasNumber: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf(`TEST-MIB DEFINITIONS ::= BEGIN
root OBJECT IDENTIFIER ::= { iso 3 }
obj OBJECT-TYPE
    SYNTAX OBJECT IDENTIFIER
    ACCESS read-only
    STATUS mandatory
    DESCRIPTION "OID default forms."
    DEFVAL { { %s } }
    ::= { root 1 }
END
`, tc.content)
			mods := parseMods(t, source)
			components := mods[0].ObjectTypes[0].DefVal.(*module.DefValOidValue).Components
			if len(components) != len(tc.want) {
				t.Fatalf("OID components = %+v, want %+v", components, tc.want)
			}
			for i, want := range tc.want {
				got := components[i]
				got.Span = types.Span{}
				testutil.Equal(t, want, got, "OID component")
			}
		})
	}
}
