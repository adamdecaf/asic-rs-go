// Package asicrs is a Go binding for [asic-rs], an ASIC miner management library.
//
// It wraps a small Rust FFI crate (asic-rs-ffi) that exposes asic-rs over a
// stable C ABI. Complex values (MinerData, configs) cross the boundary as JSON.
// Async work is handled inside Rust via a shared Tokio runtime, so Go callers
// see ordinary synchronous methods.
//
// # Quick start
//
//	factory := asicrs.NewFactory()
//	defer factory.Close()
//
//	miner, err := factory.GetMiner("192.168.1.42")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer miner.Close()
//
//	data, err := miner.GetData()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("%s @ %s: %.2f TH/s\n", data.DeviceInfo.Model, data.IP, data.HashrateTH())
//
// # Building
//
// This package uses cgo and requires a prebuilt libasic_rs_ffi shared library
// in asicrs/lib (produced by `make ffi` or `scripts/build-ffi.sh`). See the
// repository README for packaging this into larger Go projects.
//
// [asic-rs]: https://github.com/256foundation/asic-rs
package asicrs
