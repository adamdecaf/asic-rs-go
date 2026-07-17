.PHONY: all ffi test test-unit example-get-data example-scan clean tidy fmt

all: ffi test

## Build the Rust FFI bridge and install into asicrs/{include,lib}
ffi:
	@bash scripts/build-ffi.sh

## Run all package tests (requires ffi)
test: ffi
	go test ./asicrs/ -count=1 -v

## JSON/type unit tests only (still links FFI for Version)
test-unit: ffi
	go test ./asicrs/ -count=1 -run 'TestVersion|TestHashRate|TestDuration|TestPower|TestTuning|TestMinerData|TestFanConfig|TestFactory' -v

## Run get_data example (set ASIC_MINER_IP)
example-get-data: ffi
	go run ./examples/get_data

## Run scan example (set ASIC_SUBNET)
example-scan: ffi
	go run ./examples/scan

tidy:
	go mod tidy
	cd asic-rs-ffi && cargo fetch

fmt:
	gofmt -w asicrs examples
	cd asic-rs-ffi && cargo fmt

clean:
	rm -rf asic-rs-ffi/target
	rm -f asicrs/lib/libasic_rs_ffi.* asicrs/lib/asic_rs_ffi.*
