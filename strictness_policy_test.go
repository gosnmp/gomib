package gomib_test

import (
	"testing"

	"github.com/golangsnmp/gomib/internal/testutil"
	"github.com/golangsnmp/gomib/mib"
)

// resolve_strictness_policy_test.go covers the resolver-level fallback policy
// and a few compact OID cases that pin down those boundaries.

func TestResolverStrictnessBoundaries(t *testing.T) {
	tests := []struct {
		level           mib.ResolverStrictness
		wantConstrained bool
		wantGlobal      bool
	}{
		{mib.ResolverStrict, false, false},
		{mib.ResolverNormal, true, false},
		{mib.ResolverPermissive, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			testutil.Equal(t, tt.wantConstrained, tt.level.AllowConstrainedFallbacks(),
				"AllowConstrainedFallbacks at level %d", tt.level)
			testutil.Equal(t, tt.wantGlobal, tt.level.AllowGlobalFallbacks(),
				"AllowGlobalFallbacks at level %d", tt.level)
		})
	}
}

func TestOIDGlobalRootStrictness(t *testing.T) {
	tests := []struct {
		strictnessCase
		wantUnresolved bool
		wantNode       bool
	}{
		{strictnessCase: strictnessCase{name: "strict", level: mib.ResolverStrict}, wantUnresolved: true},
		{strictnessCase: strictnessCase{name: "normal", level: mib.ResolverNormal}, wantNode: true},
		{strictnessCase: strictnessCase{name: "permissive", level: mib.ResolverPermissive}, wantNode: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loadViolationMIB(t, "MISSING-IMPORT-TEST-MIB", tt.level)
			unresolvedOids := unresolvedSymbols(m, "MISSING-IMPORT-TEST-MIB", mib.UnresolvedOID)

			testutil.Equal(t, tt.wantUnresolved, unresolvedOids["enterprises"],
				"unexpected unresolved state for enterprises at %s", tt.name)

			obj := m.Object("testObject")
			testutil.NotNil(t, obj, "testObject definition at %s", tt.name)
			if obj != nil && tt.wantNode {
				testutil.NotNil(t, obj.Node(), "testObject node at %s", tt.name)
				testutil.Equal(t, "1.3.6.1.4.1.99999.1", obj.OID().String(),
					"testObject OID at %s", tt.name)
			} else if obj != nil {
				testutil.Nil(t, obj.Node(), "testObject node at %s", tt.name)
				testutil.Nil(t, obj.OID(), "testObject OID at %s", tt.name)
			}
		})
	}
}

func TestImportForwardingOidResolution(t *testing.T) {
	m := loadAtStrictness(t, "PROBLEM-FORWARDING-MIB", mib.ResolverNormal)

	obj := m.Object("problemForwardedOidObject")
	testutil.NotNil(t, obj, "Object(problemForwardedOidObject)")

	testutil.Equal(t, "1.3.6.1.4.1.99998.20.1.10", obj.OID().String(),
		"OID should resolve through forwarded parent")
}

func TestRapidCityWellKnownRootsStayDeterministic(t *testing.T) {
	for _, lvl := range allStrictnessCases {
		t.Run(lvl.name, func(t *testing.T) {
			m := loadAtStrictness(t, "RAPID-CITY", lvl.level)

			rapidCity := requireNode(t, m, "rapidCity")
			testutil.Equal(t, "1.3.6.1.4.1.2272", rapidCity.OID().String(),
				"rapidCity OID at %s", lvl.name)

			unresolved := unresolvedSymbols(m, "RAPID-CITY", mib.UnresolvedOID)
			testutil.False(t, unresolved["enterprises"],
				"enterprises should not be unresolved at %s", lvl.name)
			testutil.False(t, unresolved["internet"],
				"internet should not be unresolved at %s", lvl.name)

			for _, name := range []string{"internet", "snmpV2", "snmpModules"} {
				t.Run(name, func(t *testing.T) {
					node := requireNode(t, m, name)
					mod := node.Module()
					testutil.NotNil(t, mod, "Module() for %s", name)
					if mod != nil {
						testutil.True(t, mod.IsBase(),
							"module for %s should stay in a base module at %s, got %s",
							name, lvl.name, mod.Name())
					}
				})
			}
		})
	}
}
