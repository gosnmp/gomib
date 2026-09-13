package smiwrite

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/golangsnmp/gomib"
	"github.com/golangsnmp/gomib/mib"
)

func TestUnresolvedMetadataBeforeOutput(t *testing.T) {
	for _, tc := range []struct {
		name, definition string
		conformance      bool
	}{
		{"type", "Bad ::= INTEGER { bad(9223372036854775808), good(1) }", false},
		{"oid", `bad OBJECT-TYPE SYNTAX INTEGER MAX-ACCESS read-only STATUS current DESCRIPTION "Bad." ::= { root 4294967296 }`, false},
		{"object", `bad OBJECT-TYPE SYNTAX BITS { bad(-1), good(1) } MAX-ACCESS read-only STATUS current DESCRIPTION "Bad." ::= { root 2 }`, false},
		{"default", `bad OBJECT-TYPE SYNTAX INTEGER MAX-ACCESS read-only STATUS current DESCRIPTION "Bad." DEFVAL { 18446744073709551616 } ::= { root 2 }`, false},
		{"negative-oid-default", `bad OBJECT-TYPE SYNTAX OBJECT IDENTIFIER MAX-ACCESS read-only STATUS current DESCRIPTION "Bad." DEFVAL { { root -4294967296 2 } } ::= { root 2 }`, false},
		{"variation-default", `cap AGENT-CAPABILITIES PRODUCT-RELEASE "1" STATUS current DESCRIPTION "Capability." SUPPORTS TEST-MIB INCLUDES { } VARIATION value DEFVAL { 18446744073709551616 } DESCRIPTION "Bad." ::= { root 3 }`, true},
		{"variation-syntax", `cap AGENT-CAPABILITIES PRODUCT-RELEASE "1" STATUS current DESCRIPTION "Capability." SUPPORTS TEST-MIB INCLUDES { } VARIATION value SYNTAX INTEGER { bad(9223372036854775808) } DESCRIPTION "Bad." ::= { root 3 }`, true},
		{"variation-write", `cap AGENT-CAPABILITIES PRODUCT-RELEASE "1" STATUS current DESCRIPTION "Capability." SUPPORTS TEST-MIB INCLUDES { } VARIATION value WRITE-SYNTAX BITS { bad(-1) } DESCRIPTION "Bad." ::= { root 3 }`, true},
		{"compliance-syntax", `comp MODULE-COMPLIANCE STATUS current DESCRIPTION "Compliance." MODULE OBJECT value SYNTAX BITS { bad(-1) } DESCRIPTION "Bad." ::= { root 3 }`, true},
		{"compliance-write", `comp MODULE-COMPLIANCE STATUS current DESCRIPTION "Compliance." MODULE OBJECT value WRITE-SYNTAX INTEGER { bad(9223372036854775808) } DESCRIPTION "Bad." ::= { root 3 }`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, source := loadRecoveryWriterMib(t, tc.definition)
			var buf bytes.Buffer
			if err := Write(&buf, m, "TEST-MIB"); err == nil || !strings.Contains(err.Error(), "unresolved") || buf.Len() != 0 {
				t.Fatalf("Write = %v, output %q", err, buf.String())
			}
			if err := WriteAll(&buf, m, []string{"SNMPv2-SMI", "TEST-MIB"}); err == nil || buf.Len() != 0 {
				t.Fatalf("WriteAll = %v, output length %d", err, buf.Len())
			}
			if tc.conformance {
				if err := Write(&buf, m, "TEST-MIB", WithConformance(false)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "variation-default" {
				v := m.Module("TEST-MIB").Capabilities()[0].Supports()[0].ObjectVariations[0]
				if got := source[v.DefValSpan.Start:v.DefValSpan.End]; got != "DEFVAL { 18446744073709551616 }" || !v.DefVal.IsZero() {
					t.Fatalf("default source = %q, value = %v", got, v.DefVal)
				}
			}
		})
	}
}

func TestVariationValidNumericDefaults(t *testing.T) {
	for _, value := range []string{"0", "18446744073709551615"} {
		t.Run(value, func(t *testing.T) {
			m, _ := loadRecoveryWriterMib(t, `cap AGENT-CAPABILITIES PRODUCT-RELEASE "1" STATUS current DESCRIPTION "Capability." SUPPORTS TEST-MIB INCLUDES { } VARIATION value DEFVAL { `+value+` } DESCRIPTION "Valid." ::= { root 3 }`)
			v := m.Module("TEST-MIB").Capabilities()[0].Supports()[0].ObjectVariations[0]
			if v.DefVal.IsZero() || v.DefVal.String() != value || v.DefValSpan.Start == v.DefValSpan.End {
				t.Fatalf("variation default = %+v", v)
			}
			var buf bytes.Buffer
			if err := Write(&buf, m, "TEST-MIB"); err != nil || !strings.Contains(buf.String(), "DEFVAL { "+value+" }") {
				t.Fatalf("Write = %v, output %q", err, buf.String())
			}
		})
	}
}

func loadRecoveryWriterMib(t *testing.T, definition string) (*mib.Mib, string) {
	t.Helper()
	source := `TEST-MIB DEFINITIONS ::= BEGIN
IMPORTS OBJECT-TYPE, enterprises, Counter64 FROM SNMPv2-SMI
    AGENT-CAPABILITIES, MODULE-COMPLIANCE FROM SNMPv2-CONF;
root OBJECT IDENTIFIER ::= { enterprises 99998 }
value OBJECT-TYPE SYNTAX Counter64 MAX-ACCESS read-only STATUS current DESCRIPTION "Value." ::= { root 1 }
` + definition + "\nEND\n"
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/TEST-MIB", []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err := gomib.Dir(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := mib.SilentConfig()
	cfg.FailAt = mib.SeverityFatal
	m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("TEST-MIB"), gomib.WithDiagnosticConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	return m, source
}
