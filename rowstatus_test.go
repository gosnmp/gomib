package gomib_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/golangsnmp/gomib"
	"github.com/golangsnmp/gomib/internal/testutil"
	"github.com/golangsnmp/gomib/internal/types"
	"github.com/golangsnmp/gomib/mib"
)

func TestLoadRowStatusLanguageChecks(t *testing.T) {
	for _, tt := range []struct {
		name        string
		imports     string
		language    mib.Language
		clause      string
		validAccess string
	}{
		{"v1", "enterprises FROM RFC1155-SMI", mib.LanguageSMIv1, "ACCESS", "read-write"},
		{"v2", "enterprises FROM SNMPv2-SMI", mib.LanguageSMIv2, "MAX-ACCESS", "read-create"},
		{"unknown", "", mib.LanguageUnknown, "MAX-ACCESS", ""},
		{"mixed", "enterprises FROM RFC1155-SMI Integer32 FROM SNMPv2-SMI", mib.LanguageUnknown, "MAX-ACCESS", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Forward RowStatus through a vendor module so its defining module's
			// SMIv2 identity does not supply language evidence for this module.
			var input strings.Builder
			input.WriteString("ROW-STATUS-MIB DEFINITIONS ::= BEGIN\nIMPORTS RowStatus FROM ROW-TYPES-MIB " + tt.imports + ";\n")
			accesses := []string{"read-only", "read-write", "read-create"}
			wantAccess := make(map[string]bool)
			for i, access := range accesses {
				name := fmt.Sprintf("rowStatus%d", i)
				fmt.Fprintf(&input, `%s OBJECT-TYPE
    SYNTAX RowStatus
    %s %s
    STATUS current
    DESCRIPTION "Row status."
    DEFVAL { createAndGo }
    ::= { iso 99998 %d }
`, name, tt.clause, access, i+1)
				if tt.validAccess != "" && access != tt.validAccess {
					wantAccess[fmt.Sprintf("%q: RowStatus should have %s %s, has %s", name, tt.clause, tt.validAccess, access)] = true
				}
			}
			input.WriteString("END\n")
			src := &fakeSource{modules: map[string]fakeModule{
				"ROW-STATUS-MIB": {content: []byte(input.String())},
				"ROW-TYPES-MIB": {content: []byte(`ROW-TYPES-MIB DEFINITIONS ::= BEGIN
IMPORTS RowStatus FROM SNMPv2-TC;
END
`)},
			}}
			cfg := mib.VerboseConfig()
			cfg.FailAt = mib.SeverityFatal
			m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("ROW-STATUS-MIB"), gomib.WithDiagnosticConfig(cfg))
			testutil.NoError(t, err, "load RowStatus language fixture")
			mod := m.Module("ROW-STATUS-MIB")
			if mod == nil {
				t.Fatal("missing loaded module")
			}
			testutil.Equal(t, tt.language, mod.Language(), "detected language")
			for i := range accesses {
				obj := requireObject(t, m, fmt.Sprintf("rowStatus%d", i))
				testutil.Equal(t, m.Type("RowStatus"), obj.Type(), "resolved RowStatus identity")
				testutil.Equal(t, "createAndGo", obj.DefaultValue().String(), "resolved action default")
			}
			var defaults int
			for _, diag := range m.Diagnostics() {
				switch diag.Code {
				case types.DiagRowStatusAccess:
					if diag.Module != "ROW-STATUS-MIB" || !wantAccess[diag.Message] {
						t.Errorf("unexpected access diagnostic: %+v", diag)
					}
					delete(wantAccess, diag.Message)
				case types.DiagRowStatusDefault:
					if diag.Module == "ROW-STATUS-MIB" {
						defaults++
					}
				}
			}
			for message := range wantAccess {
				t.Errorf("missing access diagnostic: %s", message)
			}
			testutil.Equal(t, len(accesses), defaults, "language-independent action default diagnostics")
		})
	}
}
