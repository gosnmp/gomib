package gomib_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golangsnmp/gomib"
	"github.com/golangsnmp/gomib/internal/types"
	"github.com/golangsnmp/gomib/mib"
)

func TestLoadEnumZeroDiagnosticPolicy(t *testing.T) {
	const input = `ZERO-MIB DEFINITIONS ::= BEGIN
IMPORTS enterprises FROM RFC1155-SMI;
Zero ::= INTEGER { zero(0), one(1) }
END
`
	for _, ignored := range []bool{false, true} {
		t.Run(fmt.Sprintf("ignored=%t", ignored), func(t *testing.T) {
			cfg := mib.SilentConfig()
			cfg.FailAt = mib.SeveritySevere
			cfg.Overrides = map[string]mib.Severity{types.DiagEnumZero: mib.SeveritySevere}
			if ignored {
				cfg.Ignore = []string{types.DiagEnumZero}
			}
			src := &fakeSource{modules: map[string]fakeModule{"ZERO-MIB": {content: []byte(input)}}}
			m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("ZERO-MIB"), gomib.WithDiagnosticConfig(cfg))
			if m == nil {
				t.Fatalf("load returned no model: %v", err)
			}
			if ignored {
				if err != nil || len(m.Diagnostics()) != 0 {
					t.Fatalf("suppressed load = %v, diagnostics = %+v", err, m.Diagnostics())
				}
			} else {
				if !errors.Is(err, gomib.ErrDiagnosticThreshold) {
					t.Fatalf("load error = %v, want diagnostic threshold", err)
				}
				diags := m.Diagnostics()
				if len(diags) != 1 || diags[0].Code != types.DiagEnumZero || diags[0].Severity != mib.SeveritySevere || diags[0].Module != "ZERO-MIB" {
					t.Fatalf("rejection explanation = %+v", diags)
				}
			}
			typ := m.Type("Zero")
			if typ == nil {
				t.Fatal("missing enumeration")
			}
			zero, ok := typ.Enum("zero")
			if !ok || zero.Value != 0 || len(typ.Enums()) != 2 {
				t.Fatalf("retained enumeration = %+v", typ.Enums())
			}
		})
	}
}
