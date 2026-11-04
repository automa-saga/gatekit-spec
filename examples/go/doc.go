// Package examples holds runnable, real-API examples cited by the gatekit course
// (the docsify site under ../../docs). Each Example function is executed by
// `go test` and its output is verified against the `// Output:` block, so the
// snippets shown in the lessons stay correct against the released gatekit API.
//
// These examples require the gatekit module. They compile and run once that module
// is resolvable (public and/or tagged):
//
//	cd examples/go && go test ./...
package examples
