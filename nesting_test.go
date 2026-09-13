package gomib_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/golangsnmp/gomib"
	"github.com/golangsnmp/gomib/mib"
	"github.com/golangsnmp/gomib/syntax"
)

func TestRecursiveTypeInputsAreContained(t *testing.T) {
	const child = "GOMIB_NESTING_TEST_CHILD"
	if os.Getenv(child) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRecursiveTypeInputsAreContained$")
		cmd.Env = append(os.Environ(), child+"=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("nesting subprocess: %v\n%s", err, output)
		}
		return
	}
	for _, shape := range []string{"tag", "sequence", "choice", "mixed", "constrained"} {
		for _, depth := range []int{0, 63, 64, 1000, 10000} {
			t.Run(fmt.Sprintf("%s/%d", shape, depth), func(t *testing.T) {
				var prefix, suffix strings.Builder
				for i := range depth {
					open, close := "[0] ", ""
					switch shape {
					case "sequence":
						open, close = "SEQUENCE { field ", " }"
					case "choice":
						open, close = "CHOICE { field ", " }"
					case "constrained":
						close = " (0..10)"
					case "mixed":
						switch i % 3 {
						case 1:
							open, close = "SEQUENCE { field ", " }"
						case 2:
							open, close = "CHOICE { field ", " }"
						}
					}
					prefix.WriteString(open)
					suffix.WriteString(close)
				}
				input := fmt.Sprintf("DEEP-MIB DEFINITIONS ::= BEGIN\nDeepType ::= %sINTEGER%s\nIpAddress ::= OCTET STRING (SIZE (4))\nGoodType ::= INTEGER (0..10)\ngood OBJECT IDENTIFIER ::= { iso 99999 }\nEND\n", prefix.String(), suffix.String())
				limited := depth >= 64
				checkNestingSyntax(t, []byte(input), limited)
				src := &fakeSource{modules: map[string]fakeModule{"DEEP-MIB": {content: []byte(input)}}}
				m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("DEEP-MIB"))
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, d := range m.Diagnostics() {
					if d.Code == "type-nesting-limit" {
						count++
					}
				}
				want := 0
				if limited {
					want = 1
				}
				if count != want {
					t.Fatalf("limit diagnostics: got %d, want %d", count, want)
				}
				if m.Node("good") == nil || m.Node("good").OID().String() != "1.99999" {
					t.Fatal("lost surrounding OID")
				}
				if m.Type("GoodType") == nil {
					t.Fatal("lost subsequent type")
				}
				ipAddress := m.Module("DEEP-MIB").Type("IpAddress")
				if ipAddress == nil || ipAddress.Module() == nil || ipAddress.Module().Name() != "DEEP-MIB" {
					t.Fatal("lost subsequent type-keyword definition")
				}
				if limited && m.Type("DeepType") != nil {
					t.Fatal("published limited type")
				}
				if limited {
					partial := []byte("DEEP-MIB DEFINITIONS ::= BEGIN\nDeepType ::= " + prefix.String() + "-- partial\n")
					partial = append(partial, 0xff)
					checkNestingSyntax(t, partial, true)
				}
			})
		}
	}
}

func checkNestingSyntax(t *testing.T, input []byte, limited bool) {
	t.Helper()
	file, diags := syntax.Parse(input)
	if !bytes.Equal(syntax.ReconstructText(file, input), input) {
		t.Fatal("CST lost source bytes")
	}
	count := 0
	for _, d := range diags {
		if d.Code == "type-nesting-limit" {
			count++
		}
		if d.Span.Start > d.Span.End || int(d.Span.End) > len(input) {
			t.Fatal("invalid diagnostic range")
		}
	}
	want := 0
	if limited {
		want = 1
	}
	if count != want {
		t.Fatalf("syntax limit diagnostics: got %d, want %d", count, want)
	}
	file.WalkTokens(func(tok syntax.SyntaxToken) {
		if tok.Span.Start > tok.Span.End || int(tok.Span.End) > len(input) {
			t.Fatal("invalid token range")
		}
	})
}

func TestNestingLimitPolicyDoesNotPublishPartialObjects(t *testing.T) {
	input := "DEEP-MIB DEFINITIONS ::= BEGIN\nbad OBJECT-TYPE\nSYNTAX " + strings.Repeat("[0] ", 64) + "INTEGER\nMAX-ACCESS read-only\nSTATUS current\nDESCRIPTION \"limited\"\n::= { iso 99998 }\ngood OBJECT IDENTIFIER ::= { iso 99999 }\nEND\n"
	for _, ignored := range []bool{false, true} {
		cfg := mib.SilentConfig()
		cfg.FailAt = mib.SeverityError
		if ignored {
			cfg.Ignore = []string{"type-nesting-limit"}
		}
		src := &fakeSource{modules: map[string]fakeModule{"DEEP-MIB": {content: []byte(input)}}}
		m, err := gomib.Load(context.Background(), gomib.WithSource(src), gomib.WithModules("DEEP-MIB"), gomib.WithDiagnosticConfig(cfg))
		if ignored {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			if !errors.Is(err, gomib.ErrDiagnosticThreshold) {
				t.Fatalf("expected threshold rejection, got %v", err)
			}
			diags := m.Diagnostics()
			if len(diags) != 1 || diags[0].Code != "type-nesting-limit" || diags[0].Line != 3 || diags[0].Column != 264 || diags[0].EndColumn != 271 {
				t.Fatalf("unexpected limit explanation: %+v", diags)
			}
		}
		if m.Node("bad") != nil || m.Object("bad") != nil {
			t.Fatal("published incomplete object")
		}
		if m.Node("good") == nil || m.Node("good").OID().String() != "1.99999" {
			t.Fatal("lost surrounding OID")
		}
	}
}
