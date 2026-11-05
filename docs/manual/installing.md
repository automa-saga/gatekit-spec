# Installing

gatekit has two layers. Take only the one you need.

| Layer | Go import path | Use it when |
|---|---|---|
| **Gate core** | `github.com/automa-saga/gatekit` | You want to ship behavior dormant and turn it on by version. Dependency-light. |
| **Migration / activation** | `github.com/automa-saga/gatekit/migration` | You also need deferred activation, one-time migrations, or `finalize` for a rolling upgrade. Opt-in, same module. |
| **Durable runner** | `github.com/automa-saga/gatekit-automa` | You need `finalize` to survive a process crash. Separate module, so [automa](https://github.com/automa-saga/automa) never enters your module graph unless you ask for it. |

## Install

```bash
go get github.com/automa-saga/gatekit
```

Add the durable runner only if you need it:

```bash
go get github.com/automa-saga/gatekit-automa
```

## Check it works

```go
package main

import (
	"fmt"

	"github.com/automa-saga/gatekit"
)

func main() {
	gate, err := gatekit.New(gatekit.Registry{}, gatekit.StaticVersion(0))
	if err != nil {
		panic(err)
	}
	fmt.Println(gate.Enabled("anything")) // false: an unknown feature is always off
}
```

## Other languages

Go is the only released implementation. Kotlin, Rust, and Python are planned; see
[Other languages](/tutorial/go.md).

## Next

[Declaring features](/manual/declaring-features.md)
