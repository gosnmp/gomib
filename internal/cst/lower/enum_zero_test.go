package lower_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/golangsnmp/gomib/internal/module"
	"github.com/golangsnmp/gomib/internal/types"
)

func TestEnumZeroLanguage(t *testing.T) {
	const definitions = `ZeroType ::= INTEGER { zeroType(0), one(1), negative(-1) }
ZeroConvention ::= TEXTUAL-CONVENTION
    STATUS current
    DESCRIPTION "Zero convention."
    SYNTAX INTEGER { zeroConvention(0), one(1) }
zeroObject OBJECT-TYPE
    SYNTAX INTEGER { zeroObject(0), one(1) }
    ACCESS read-only
    STATUS mandatory
    DESCRIPTION "Zero object."
    ::= { iso 3 }
ZeroBits ::= BITS { zeroBit(0) }
`
	const identity = `root MODULE-IDENTITY
    LAST-UPDATED "202609130000Z"
    ORGANIZATION "Test"
    CONTACT-INFO "Test"
    DESCRIPTION "Test"
    ::= { iso 4 }
`
	const trap = `testTrap TRAP-TYPE
    ENTERPRISE iso
    DESCRIPTION "Test"
    ::= 1
`
	for _, tc := range []struct {
		name, imports, suffix string
		language              types.Language
	}{
		{"v1", "IMPORTS enterprises FROM RFC1155-SMI;", "", types.LanguageSMIv1},
		{"v2", "IMPORTS enterprises FROM SNMPv2-SMI;", "", types.LanguageSMIv2},
		{"unknown", "", "", types.LanguageUnknown},
		{"mixed imports", "IMPORTS enterprises FROM RFC1155-SMI Integer32 FROM SNMPv2-SMI;", "", types.LanguageUnknown},
		{"later v1 syntax", "", trap, types.LanguageSMIv1},
		{"later v2 syntax", "", identity, types.LanguageSMIv2},
		{"later conflicting syntax", "IMPORTS enterprises FROM RFC1155-SMI;", identity, types.LanguageUnknown},
		{"mixed syntax", "", identity + trap, types.LanguageUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "ZERO-MIB DEFINITIONS ::= BEGIN\n" + tc.imports + "\n" + definitions + tc.suffix + "END\n"
			mods := parseMods(t, source)
			if len(mods) != 1 || mods[0].Language != tc.language {
				t.Fatalf("modules = %+v, want one with language %v", mods, tc.language)
			}
			mod := mods[0]
			if len(mod.TypeDefs) != 3 || len(mod.ObjectTypes) != 1 {
				t.Fatalf("retained %d types and %d objects", len(mod.TypeDefs), len(mod.ObjectTypes))
			}
			for _, syntax := range []module.TypeSyntax{mod.TypeDefs[0].Syntax, mod.TypeDefs[1].Syntax, mod.ObjectTypes[0].Syntax} {
				enum := syntax.(*module.TypeSyntaxIntegerEnum)
				if len(enum.NamedNumbers) < 2 || enum.NamedNumbers[0].Value != 0 || enum.NamedNumbers[1].Value != 1 {
					t.Fatalf("enum changed: %+v", enum.NamedNumbers)
				}
			}
			var got []types.Diagnostic
			for _, diag := range mod.Diagnostics {
				if diag.Code == types.DiagEnumZero {
					got = append(got, diag)
				}
			}
			want := 0
			if tc.language == types.LanguageSMIv1 {
				want = 3
			}
			if len(got) != want {
				t.Fatalf("enum-zero diagnostics = %+v, want %d", got, want)
			}
			lines := strings.Split(source, "\n")
			for i, diag := range got {
				name := []string{"zeroType", "zeroConvention", "zeroObject"}[i]
				if diag.Module != "ZERO-MIB" || diag.Severity != types.SeverityError || diag.Line != diag.EndLine || diag.Message != fmt.Sprintf("enumeration contains zero value %q(0) in SMIv1 module", name) {
					t.Errorf("unexpected diagnostic: %+v", diag)
				}
				if token := lines[diag.Line-1][diag.Column-1 : diag.EndColumn-1]; token != name+"(0)" {
					t.Errorf("diagnostic source = %q, want %s(0)", token, name)
				}
			}
		})
	}
}

func TestEnumZeroModuleIsolation(t *testing.T) {
	const source = `FIRST-MIB DEFINITIONS ::= BEGIN
IMPORTS enterprises FROM RFC1155-SMI;
First ::= INTEGER { first(0) }
END
UNKNOWN-MIB DEFINITIONS ::= BEGIN
Unknown ::= INTEGER { unknown(0) }
END
LAST-MIB DEFINITIONS ::= BEGIN
IMPORTS enterprises FROM RFC1155-SMI;
Last ::= INTEGER { last(0) }
END
`
	mods := parseMods(t, source)
	if len(mods) != 3 {
		t.Fatalf("modules = %d", len(mods))
	}
	for i, mod := range mods {
		count := 0
		for _, diag := range mod.Diagnostics {
			if diag.Code == types.DiagEnumZero {
				count++
				if diag.Module != mod.Name || !strings.Contains(diag.Message, []string{"first", "unknown", "last"}[i]) {
					t.Errorf("leaked diagnostic: %+v", diag)
				}
			}
		}
		if count != []int{1, 0, 1}[i] {
			t.Errorf("%s: got %d enum-zero diagnostics", mod.Name, count)
		}
	}
}

func TestEnumZeroDiagnosticPolicy(t *testing.T) {
	const source = `ZERO-MIB DEFINITIONS ::= BEGIN
IMPORTS enterprises FROM RFC1155-SMI;
Zero ::= INTEGER { zero(0) }
END
`
	for _, ignored := range []bool{false, true} {
		t.Run(fmt.Sprintf("ignored=%t", ignored), func(t *testing.T) {
			cfg := types.VerboseConfig()
			cfg.Overrides = map[string]types.Severity{types.DiagEnumZero: types.SeverityWarning}
			if ignored {
				cfg.Ignore = []string{types.DiagEnumZero}
			}
			mods := parseModsWithConfig(t, source, cfg)
			diag := findDiag(mods, types.DiagEnumZero)
			if ignored {
				if diag != nil {
					t.Fatalf("ignored diagnostic retained: %+v", diag)
				}
			} else if diag == nil || diag.Severity != types.SeverityWarning {
				t.Fatalf("override diagnostic = %+v", diag)
			}
		})
	}
}
