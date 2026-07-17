package asicrs

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo LDFLAGS: -L${SRCDIR}/lib -lasic_rs_ffi
#cgo darwin LDFLAGS: -Wl,-rpath,${SRCDIR}/lib -framework Security -framework CoreFoundation
#cgo linux LDFLAGS: -Wl,-rpath,${SRCDIR}/lib -lm -ldl -lpthread
#cgo windows LDFLAGS: -lws2_32 -luserenv -lbcrypt -lntdll

#include "asic_rs_ffi.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// lastError returns the thread-local error from the Rust FFI, if any.
func lastError() error {
	cstr := C.asic_rs_last_error()
	if cstr == nil {
		return fmt.Errorf("unknown asic-rs error")
	}
	return fmt.Errorf("%s", C.GoString(cstr))
}

// freeCString releases a string allocated by the FFI.
func freeCString(s *C.char) {
	if s != nil {
		C.asic_rs_free_string(s)
	}
}

// goStringOwned copies a C string owned by the FFI and frees it.
func goStringOwned(s *C.char) string {
	if s == nil {
		return ""
	}
	defer freeCString(s)
	return C.GoString(s)
}

// takeJSON unmarshals a FFI-owned JSON string into dest and frees it.
func takeJSON(s *C.char, dest any) error {
	if s == nil {
		return lastError()
	}
	defer freeCString(s)
	if err := json.Unmarshal([]byte(C.GoString(s)), dest); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

// takeJSONBytes returns raw JSON bytes from an FFI-owned string.
func takeJSONBytes(s *C.char) ([]byte, error) {
	if s == nil {
		return nil, lastError()
	}
	defer freeCString(s)
	return []byte(C.GoString(s)), nil
}

// cString allocates a C string; caller must free with C.free.
func cString(s string) *C.char {
	return C.CString(s)
}

func freeGoCString(s *C.char) {
	if s != nil {
		C.free(unsafe.Pointer(s))
	}
}

// controlResult maps the FFI 1/0/-1 control convention to Go.
func controlResult(code C.int) (bool, error) {
	switch code {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, lastError()
	}
}
