# asic-rs-go

Go bindings for [asic-rs](https://github.com/256foundation/asic-rs) — discover, monitor, and control ASIC miners from Go.

This repository is a **library** meant to be imported by larger Go services (fleet monitors, dashboards, ops tools). It is not a standalone CLI product, though small examples are included.

## Architecture

```
┌─────────────────┐     cgo      ┌──────────────────┐     Rust     ┌──────────┐
│  your Go app    │ ───────────► │  asicrs (Go)     │ ───────────► │ asic-rs  │
│                 │              │  + libasic_rs_ffi│  Tokio/async │ backends │
└─────────────────┘              └──────────────────┘              └──────────┘
```

| Layer | Role |
|-------|------|
| **`asicrs`** | Idiomatic Go API (`Factory`, `Miner`, typed `MinerData`) |
| **`asic-rs-ffi`** | Thin Rust crate: `extern "C"` ABI, opaque handles, JSON for complex values |
| **`asic-rs` 0.7.x** | Upstream miner discovery, telemetry, and control |

Design choices:

- **JSON over the FFI boundary** for `MinerData`, configs, and nested types — avoids fragile C structs and matches asic-rs’s serde models.
- **Synchronous Go API** — a multi-thread Tokio runtime lives inside the FFI and `block_on`s async work.
- **Opaque handles** for factory/miner ownership; always `Close()` (finalizers are a safety net only).
- **cgo** for linking (simplest path; `CGO_ENABLED=1` required).

## Requirements

- Go 1.22+ (module targets current toolchain)
- Rust stable (`cargo`, `rustc`) to build the FFI
- C toolchain (Xcode CLT on macOS, `build-essential` on Linux)
- Network access to miners for integration use

## Quick start (this repo)

```bash
# 1. Build the native library into asicrs/lib
make ffi

# 2. Unit / library tests (no miners required for factory host-list tests)
make test

# 3. Optional: talk to a real device
ASIC_MINER_IP=192.168.1.42 go run ./examples/get_data
ASIC_SUBNET=192.168.1.0/24 go run ./examples/scan
```

## Use as a dependency

```bash
go get github.com/adamdecaf/asic-rs-go/asicrs@latest
```

```go
package main

import (
    "fmt"
    "log"

    "github.com/adamdecaf/asic-rs-go/asicrs"
)

func main() {
    factory := asicrs.NewFactory()
    defer factory.Close()

    miner, err := factory.GetMiner("192.168.1.42")
    if err != nil {
        log.Fatal(err)
    }
    defer miner.Close()

    data, err := miner.GetData()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%s: %.2f TH/s mining=%v\n",
        data.DeviceInfo.Model, data.HashrateTH(), data.IsMining)
}
```

### Important: native library packaging

`go get` pulls **Go sources**, not the prebuilt `libasic_rs_ffi`. Consumers must either:

1. **Build FFI in their module** (recommended for apps you control):

   ```bash
   # from a checkout of asic-rs-go, or a vendored copy of asic-rs-ffi
   make -C path/to/asic-rs-go ffi
   ```

   Point cgo at the artifacts with env vars if you install them outside the package tree:

   ```bash
   export CGO_CFLAGS="-I/path/to/include"
   export CGO_LDFLAGS="-L/path/to/lib -lasic_rs_ffi"
   # macOS runtime search path
   export DYLD_LIBRARY_PATH=/path/to/lib   # or install_name_tool / rpath
   ```

2. **Vendor the whole repo** (including `asic-rs-ffi` + built `asicrs/lib`) into your monorepo and `replace` the module path.

3. **Ship platform binaries**: build `libasic_rs_ffi` per target (`x86_64-unknown-linux-gnu`, `aarch64-apple-darwin`, …), attach them to releases, and download in CI before `go build`.

The default cgo directives look for libraries under `asicrs/lib` relative to the package source (`${SRCDIR}/lib`) and set an rpath on macOS/Linux so the dylib/so is found next to the package during development.

### Cross-compilation

cgo + Rust native code must be built for the **same** GOOS/GOARCH as the final binary. Typical pattern:

```bash
# Example: linux/amd64 from a suitably tooled host
cd asic-rs-ffi
cargo build --release --target x86_64-unknown-linux-gnu
# copy target/x86_64-unknown-linux-gnu/release/libasic_rs_ffi.so → asicrs/lib
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build ./...
```

Static linking is possible via the produced `libasic_rs_ffi.a`, but you must also link transitive system deps (on macOS: Security/CoreFoundation; on Linux: pthread, dl, m, and often the Rust standard runtime already folded into the archive). Prefer the shared library unless you have a strong reason to static-link.

## API overview

### Factory (discovery)

| Method | Description |
|--------|-------------|
| `NewFactory` / `NewFactoryFromSubnet` / `FromRange` / `FromOctets` | Construct discovery scopes |
| `WithSubnet` / `WithRange` / `WithOctets` | Append hosts |
| `SetConcurrentLimit`, `SetIdentificationTimeoutSecs`, … | Tune scan behaviour |
| `GetMiner(ip)` | Identify one IP |
| `Scan()` | Scan all configured hosts → `[]*Miner` |
| `Hosts()`, `Len()`, `IsEmpty()` | Inspect host list |

### Miner (telemetry + control)

| Area | Methods |
|------|---------|
| Identity | `IP`, `DeviceInfo`, `Summary`, `Supports` |
| Auth | `SetAuth(user, pass)` |
| Full snapshot | `GetData`, `GetDataJSON` |
| Field getters | `GetHashrate`, `GetFans`, `GetPools`, `GetWattage`, … |
| Config | `Get/SetPoolsConfig`, `Scaling`, `Tuning`, `Fan` |
| Control | `Restart`, `Pause`, `Resume`, `SetPowerLimit`, `SetFaultLight`, `UpgradeFirmware` |

Complex types live in `types.go` and mirror asic-rs’s serde JSON (hashrate units, tuning targets, pool groups, etc.).

## Testing advice

| Layer | What to run | Needs miners? |
|-------|-------------|----------------|
| Pure JSON models | `go test ./asicrs -run 'TestHashRate\|TestMinerData\|…'` | No |
| FFI smoke | `TestVersion`, factory host generation | No |
| Integration | `GetMiner` / `Scan` / `GetData` against lab hardware | Yes |

Suggested CI:

```yaml
- run: make ffi
- run: go test ./asicrs/ -count=1
# optional nightly:
- run: ASIC_MINER_IP=$LAB_IP go test ./... -tags=integration
```

Mark destructive control tests (`Restart`, `UpgradeFirmware`) with build tags or explicit env guards so CI never reboots production fleets.

## Project layout

```
asic-rs-go/
├── asicrs/                 # Go package (import path …/asicrs)
│   ├── include/            # generated C header
│   ├── lib/                # libasic_rs_ffi.{dylib,so,a} (built, not always committed)
│   ├── factory.go
│   ├── miner.go
│   └── types.go
├── asic-rs-ffi/            # Rust cdylib/staticlib bridge
│   └── src/lib.rs
├── examples/
│   ├── get_data/
│   └── scan/
├── scripts/build-ffi.sh
└── Makefile
```

## Limitations / roadmap

- **No purego path yet** — cgo only; a purego loader could be added later for `CGO_ENABLED=0` with `dlopen`.
- **Scan streaming** — asic-rs exposes `scan_stream`; the Go API currently uses batch `Scan()` only.
- **Firmware registry filtering** — custom firmware lists are not yet exposed over FFI.
- **Listener** — `MinerListener` is not wrapped.

## License

Apache-2.0 (aligned with asic-rs). See upstream licenses for transitive crates.
