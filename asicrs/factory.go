package asicrs

/*
#include "asic_rs_ffi.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// Factory discovers and constructs miners on a network.
//
// Always call Close when finished (or use defer). The zero value is invalid.
type Factory struct {
	ptr *C.AsicFactory
}

// NewFactory creates an empty factory with no hosts configured.
func NewFactory() *Factory {
	f := &Factory{ptr: C.asic_rs_factory_new()}
	runtime.SetFinalizer(f, (*Factory).Close)
	return f
}

// NewFactoryFromSubnet creates a factory pre-loaded with hosts from a CIDR
// subnet (for example "192.168.1.0/24").
func NewFactoryFromSubnet(subnet string) (*Factory, error) {
	cs := cString(subnet)
	defer freeGoCString(cs)
	ptr := C.asic_rs_factory_from_subnet(cs)
	if ptr == nil {
		return nil, lastError()
	}
	f := &Factory{ptr: ptr}
	runtime.SetFinalizer(f, (*Factory).Close)
	return f, nil
}

// NewFactoryFromRange creates a factory from a compact range string
// (for example "192.168.1.1-255").
func NewFactoryFromRange(rangeStr string) (*Factory, error) {
	cs := cString(rangeStr)
	defer freeGoCString(cs)
	ptr := C.asic_rs_factory_from_range(cs)
	if ptr == nil {
		return nil, lastError()
	}
	f := &Factory{ptr: ptr}
	runtime.SetFinalizer(f, (*Factory).Close)
	return f, nil
}

// NewFactoryFromOctets creates a factory from four octet descriptors
// (for example "192", "168", "1", "1-255").
func NewFactoryFromOctets(o1, o2, o3, o4 string) (*Factory, error) {
	c1, c2, c3, c4 := cString(o1), cString(o2), cString(o3), cString(o4)
	defer freeGoCString(c1)
	defer freeGoCString(c2)
	defer freeGoCString(c3)
	defer freeGoCString(c4)
	ptr := C.asic_rs_factory_from_octets(c1, c2, c3, c4)
	if ptr == nil {
		return nil, lastError()
	}
	f := &Factory{ptr: ptr}
	runtime.SetFinalizer(f, (*Factory).Close)
	return f, nil
}

// Close frees the factory. Safe to call multiple times.
func (f *Factory) Close() {
	if f == nil || f.ptr == nil {
		return
	}
	C.asic_rs_factory_free(f.ptr)
	f.ptr = nil
	runtime.SetFinalizer(f, nil)
}

func (f *Factory) check() error {
	if f == nil || f.ptr == nil {
		return fmt.Errorf("factory is closed or nil")
	}
	return nil
}

// WithSubnet appends hosts from a CIDR subnet.
func (f *Factory) WithSubnet(subnet string) error {
	if err := f.check(); err != nil {
		return err
	}
	cs := cString(subnet)
	defer freeGoCString(cs)
	if C.asic_rs_factory_with_subnet(f.ptr, cs) != 0 {
		return lastError()
	}
	return nil
}

// WithRange appends hosts from a range string.
func (f *Factory) WithRange(rangeStr string) error {
	if err := f.check(); err != nil {
		return err
	}
	cs := cString(rangeStr)
	defer freeGoCString(cs)
	if C.asic_rs_factory_with_range(f.ptr, cs) != 0 {
		return lastError()
	}
	return nil
}

// WithOctets appends hosts from four octet descriptors.
func (f *Factory) WithOctets(o1, o2, o3, o4 string) error {
	if err := f.check(); err != nil {
		return err
	}
	c1, c2, c3, c4 := cString(o1), cString(o2), cString(o3), cString(o4)
	defer freeGoCString(c1)
	defer freeGoCString(c2)
	defer freeGoCString(c3)
	defer freeGoCString(c4)
	if C.asic_rs_factory_with_octets(f.ptr, c1, c2, c3, c4) != 0 {
		return lastError()
	}
	return nil
}

// SetPortCheck enables or disables the initial port connectivity check.
func (f *Factory) SetPortCheck(enabled bool) {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_port_check(f.ptr, C.bool(enabled))
}

// SetConcurrentLimit sets the maximum concurrent discovery tasks.
func (f *Factory) SetConcurrentLimit(limit int) {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_concurrent_limit(f.ptr, C.uintptr_t(limit))
}

// SetIdentificationTimeoutSecs sets how long identification may take.
func (f *Factory) SetIdentificationTimeoutSecs(secs uint64) {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_identification_timeout_secs(f.ptr, C.uint64_t(secs))
}

// SetConnectivityTimeoutSecs sets the TCP connect timeout for port checks.
func (f *Factory) SetConnectivityTimeoutSecs(secs uint64) {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_connectivity_timeout_secs(f.ptr, C.uint64_t(secs))
}

// SetConnectivityRetries sets how many connectivity attempts to make.
func (f *Factory) SetConnectivityRetries(retries uint32) {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_connectivity_retries(f.ptr, C.uint32_t(retries))
}

// SetNofileLimit sets a desired RLIMIT_NOFILE / maxstdio target for large scans.
func (f *Factory) SetNofileLimit(limit uint64) {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_nofile_limit(f.ptr, C.uint64_t(limit))
}

// SetNofileAdjustment enables or disables automatic nofile raising.
func (f *Factory) SetNofileAdjustment(enabled bool) {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_nofile_adjustment(f.ptr, C.bool(enabled))
}

// SetAdaptiveConcurrency picks a concurrency limit based on the host list size.
func (f *Factory) SetAdaptiveConcurrency() {
	if f.check() != nil {
		return
	}
	C.asic_rs_factory_set_adaptive_concurrency(f.ptr)
}

// Len returns the number of hosts currently configured for scanning.
func (f *Factory) Len() int {
	if f.check() != nil {
		return 0
	}
	n := C.asic_rs_factory_len(f.ptr)
	if n < 0 {
		return 0
	}
	return int(n)
}

// IsEmpty reports whether no hosts are configured.
func (f *Factory) IsEmpty() bool {
	if f.check() != nil {
		return true
	}
	return bool(C.asic_rs_factory_is_empty(f.ptr))
}

// Hosts returns the configured host IP strings.
func (f *Factory) Hosts() ([]string, error) {
	if err := f.check(); err != nil {
		return nil, err
	}
	var hosts []string
	if err := takeJSON(C.asic_rs_factory_hosts_json(f.ptr), &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

// GetMiner discovers and constructs a miner at ip.
func (f *Factory) GetMiner(ip string) (*Miner, error) {
	if err := f.check(); err != nil {
		return nil, err
	}
	cs := cString(ip)
	defer freeGoCString(cs)
	ptr := C.asic_rs_factory_get_miner(f.ptr, cs)
	if ptr == nil {
		return nil, lastError()
	}
	return newMiner(ptr), nil
}

// ScanMiner scans a single IP with the factory's port pre-check logic.
func (f *Factory) ScanMiner(ip string) (*Miner, error) {
	if err := f.check(); err != nil {
		return nil, err
	}
	cs := cString(ip)
	defer freeGoCString(cs)
	ptr := C.asic_rs_factory_scan_miner(f.ptr, cs)
	if ptr == nil {
		return nil, lastError()
	}
	return newMiner(ptr), nil
}

// Scan discovers miners on all configured hosts.
// The caller must Close each returned Miner.
func (f *Factory) Scan() ([]*Miner, error) {
	if err := f.check(); err != nil {
		return nil, err
	}
	var out **C.AsicMiner
	var n C.size_t
	if C.asic_rs_factory_scan(f.ptr, &out, &n) != 0 {
		return nil, lastError()
	}
	if n == 0 {
		if out != nil {
			C.asic_rs_free_miner_list(out, 0)
		}
		return []*Miner{}, nil
	}
	// Convert C array of pointers into Go slice of Miners.
	defer C.asic_rs_free_miner_list(out, n)
	slice := unsafe.Slice(out, int(n))
	miners := make([]*Miner, 0, int(n))
	for _, p := range slice {
		if p != nil {
			miners = append(miners, newMiner(p))
		}
	}
	return miners, nil
}
