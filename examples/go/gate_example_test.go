package examples_test

import (
	"fmt"

	"github.com/automa-saga/gatekit"
)

// SplitCatalog is declared once as a typed constant and used by both the registry
// and every call site, so a mistyped identifier can never silently resolve a
// different (unknown) feature off. See course Lesson 4, Step 1.
const SplitCatalog gatekit.Feature = "split-catalog"

// catalogRegistry is the feature's lifecycle: born dormant (Beta, default off) at
// activation version 5, promoted to GA (default on) at 6.
var catalogRegistry = gatekit.Registry{
	SplitCatalog: {
		Description: "Load the per-minor split catalog",
		Specs: gatekit.VersionedSpecs{
			{ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
			{ActiveFrom: 6, Default: true, Maturity: gatekit.GA},
		},
	},
}

// Example_dormantThenFlip shows a feature shipped dormant at activation 5 and turned
// on by the next spec at activation 6 (Lesson 4, Steps 3–4).
func Example_dormantThenFlip() {
	gate, err := gatekit.New(catalogRegistry, gatekit.StaticVersion(0))
	if err != nil {
		panic(err)
	}

	fmt.Println(gate.EnabledAt(SplitCatalog, 4)) // first spec (5) not reached yet
	fmt.Println(gate.EnabledAt(SplitCatalog, 5)) // reached, but default off: dormant
	fmt.Println(gate.EnabledAt(SplitCatalog, 6)) // 6-spec default on: live
	// Output:
	// false
	// false
	// true
}

// Example_override forces a feature on regardless of the version — the
// highest-precedence input to resolution (Lesson 4, Step 4b).
func Example_override() {
	gate, err := gatekit.New(
		catalogRegistry,
		gatekit.StaticVersion(5), // version alone would resolve off…
		gatekit.WithOverrides(map[gatekit.Feature]bool{SplitCatalog: true}),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(gate.Enabled(SplitCatalog)) // …but the override wins
	// Output: true
}

// Example_alphaOptIn shows that an ALPHA feature stays off even with Default: true,
// unless the gate is opted in (Lesson 4; Appendix A, step 3).
func Example_alphaOptIn() {
	const NewIndexer gatekit.Feature = "new-indexer"
	reg := gatekit.Registry{
		NewIndexer: {Specs: gatekit.VersionedSpecs{
			{ActiveFrom: 7, Default: true, Maturity: gatekit.Alpha},
		}},
	}

	off, _ := gatekit.New(reg, gatekit.StaticVersion(7))
	on, _ := gatekit.New(reg, gatekit.StaticVersion(7), gatekit.WithExperimental(true))

	fmt.Println(off.Enabled(NewIndexer)) // alpha off by default
	fmt.Println(on.Enabled(NewIndexer))  // opted in: follows its default
	// Output:
	// false
	// true
}

// Example_report renders the operator-facing dry-run listing (Lesson 4).
func Example_report() {
	gate, _ := gatekit.New(catalogRegistry, gatekit.StaticVersion(6))
	fmt.Print(gate.Report())
	// Output:
	// Features at activation 6:
	//   - split-catalog [GA] on  Load the per-minor split catalog
}
