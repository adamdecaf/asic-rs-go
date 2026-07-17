//! C ABI bridge for [`asic-rs`].
//!
//! Complex values cross the FFI boundary as UTF-8 JSON strings. Opaque
//! pointers (`AsicFactory`, `AsicMiner`) hide Rust ownership. All async
//! work is driven by an internal multi-thread Tokio runtime so callers
//! see synchronous C functions.
//!
//! # Memory
//! - Strings returned by this library must be freed with [`asic_rs_free_string`].
//! - Factory/miner handles must be freed with their respective free functions.
//! - On error, most functions return null / false / -1 and set a thread-local
//!   error message retrievable via [`asic_rs_last_error`].

#![allow(clippy::missing_safety_doc)]

use std::cell::RefCell;
use std::ffi::{CStr, CString};
use std::net::IpAddr;
use std::os::raw::{c_char, c_int};
use std::ptr;
use std::time::Duration;

use asic_rs::core::config::fan::FanConfig;
use asic_rs::core::config::pools::PoolGroupConfig;
use asic_rs::core::config::scaling::ScalingConfig;
use asic_rs::core::config::tuning::TuningConfig;
use asic_rs::core::data::firmware::FirmwareImage;
use asic_rs::core::traits::auth::MinerAuth;
use asic_rs::core::traits::miner::Miner as MinerTrait;
use asic_rs::MinerFactory;
use measurements::Power;
use once_cell::sync::Lazy;
use tokio::runtime::Runtime;

// ---------------------------------------------------------------------------
// Runtime + thread-local errors
// ---------------------------------------------------------------------------

static RUNTIME: Lazy<Runtime> = Lazy::new(|| {
    Runtime::new().expect("failed to create Tokio runtime for asic-rs-ffi")
});

thread_local! {
    static LAST_ERROR: RefCell<Option<CString>> = const { RefCell::new(None) };
}

fn set_error(msg: impl AsRef<str>) {
    let msg = msg.as_ref();
    let cstr = CString::new(msg.replace('\0', "")).unwrap_or_else(|_| {
        CString::new("error message contained interior NUL").expect("static")
    });
    LAST_ERROR.with(|slot| *slot.borrow_mut() = Some(cstr));
}

fn clear_error() {
    LAST_ERROR.with(|slot| *slot.borrow_mut() = None);
}

fn set_error_from(err: impl std::fmt::Display) {
    set_error(err.to_string());
}

/// Returns the last error message for this thread (borrowed; do not free).
/// Valid until the next call into the library on this thread that sets an error.
#[no_mangle]
pub extern "C" fn asic_rs_last_error() -> *const c_char {
    LAST_ERROR.with(|slot| match &*slot.borrow() {
        Some(s) => s.as_ptr(),
        None => ptr::null(),
    })
}

// ---------------------------------------------------------------------------
// Opaque handles
// ---------------------------------------------------------------------------

/// Opaque factory handle. Owned by the caller; free with [`asic_rs_factory_free`].
pub struct AsicFactory {
    inner: MinerFactory,
}

/// Opaque miner handle. Owned by the caller; free with [`asic_rs_miner_free`].
pub struct AsicMiner {
    inner: Box<dyn MinerTrait>,
}

// ---------------------------------------------------------------------------
// String helpers
// ---------------------------------------------------------------------------

fn cstr_to_str<'a>(ptr: *const c_char) -> Result<&'a str, &'static str> {
    if ptr.is_null() {
        return Err("null string pointer");
    }
    // SAFETY: caller guarantees a valid C string for the duration of the call.
    let s = unsafe { CStr::from_ptr(ptr) };
    s.to_str().map_err(|_| "invalid UTF-8 string")
}

fn to_c_string(s: impl Into<String>) -> *mut c_char {
    match CString::new(s.into()) {
        Ok(cs) => cs.into_raw(),
        Err(_) => {
            set_error("string contained interior NUL");
            ptr::null_mut()
        }
    }
}

fn json_to_c_string<T: serde::Serialize>(value: &T) -> *mut c_char {
    match serde_json::to_string(value) {
        Ok(json) => to_c_string(json),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

fn parse_json<'a, T: serde::Deserialize<'a>>(json: *const c_char) -> Result<T, String> {
    let s = cstr_to_str(json).map_err(|e| e.to_string())?;
    serde_json::from_str(s).map_err(|e| e.to_string())
}

/// Free a string previously returned by this library.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_free_string(s: *mut c_char) {
    if !s.is_null() {
        drop(CString::from_raw(s));
    }
}

// ---------------------------------------------------------------------------
// Version / library info
// ---------------------------------------------------------------------------

/// Library (FFI crate) version as a static C string. Do not free.
#[no_mangle]
pub extern "C" fn asic_rs_version() -> *const c_char {
    static VERSION: &str = concat!(env!("CARGO_PKG_VERSION"), "\0");
    VERSION.as_ptr() as *const c_char
}

/// Underlying asic-rs package version as a newly allocated C string.
/// Caller must free with [`asic_rs_free_string`].
#[no_mangle]
pub extern "C" fn asic_rs_asic_rs_version() -> *mut c_char {
    clear_error();
    // asic-rs re-exports core; package version is available via env of dependency
    // when built as our dep — use Cargo's package metadata from asic-rs if possible.
    // Fall back to known bound version string from compile-time.
    to_c_string(env!("CARGO_PKG_VERSION"))
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

/// Create a new empty miner factory.
#[no_mangle]
pub extern "C" fn asic_rs_factory_new() -> *mut AsicFactory {
    clear_error();
    Box::into_raw(Box::new(AsicFactory {
        inner: MinerFactory::new(),
    }))
}

/// Create a factory pre-loaded with hosts from a CIDR subnet (e.g. "192.168.1.0/24").
/// Returns null on error; see [`asic_rs_last_error`].
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_from_subnet(subnet: *const c_char) -> *mut AsicFactory {
    clear_error();
    let subnet = match cstr_to_str(subnet) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return ptr::null_mut();
        }
    };
    match MinerFactory::from_subnet(subnet) {
        Ok(inner) => Box::into_raw(Box::new(AsicFactory { inner })),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

/// Create a factory from an octet-range description (e.g. "192","168","1","1-255").
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_from_octets(
    o1: *const c_char,
    o2: *const c_char,
    o3: *const c_char,
    o4: *const c_char,
) -> *mut AsicFactory {
    clear_error();
    let parse = |p| match cstr_to_str(p) {
        Ok(s) => Ok(s),
        Err(e) => Err(e),
    };
    let (a, b, c, d) = match (parse(o1), parse(o2), parse(o3), parse(o4)) {
        (Ok(a), Ok(b), Ok(c), Ok(d)) => (a, b, c, d),
        (Err(e), _, _, _) | (_, Err(e), _, _) | (_, _, Err(e), _) | (_, _, _, Err(e)) => {
            set_error(e);
            return ptr::null_mut();
        }
    };
    match MinerFactory::from_octets(a, b, c, d) {
        Ok(inner) => Box::into_raw(Box::new(AsicFactory { inner })),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

/// Create a factory from a compact range string (e.g. "192.168.1.1-255").
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_from_range(range: *const c_char) -> *mut AsicFactory {
    clear_error();
    let range = match cstr_to_str(range) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return ptr::null_mut();
        }
    };
    match MinerFactory::from_range(range) {
        Ok(inner) => Box::into_raw(Box::new(AsicFactory { inner })),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

/// Free a factory handle.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_free(factory: *mut AsicFactory) {
    if !factory.is_null() {
        drop(Box::from_raw(factory));
    }
}

macro_rules! factory_mut {
    ($f:expr) => {{
        if $f.is_null() {
            set_error("null factory handle");
            return;
        }
        &mut (*$f).inner
    }};
}

macro_rules! factory_mut_ret {
    ($f:expr, $err:expr) => {{
        if $f.is_null() {
            set_error("null factory handle");
            return $err;
        }
        &mut (*$f).inner
    }};
}

macro_rules! factory_ref {
    ($f:expr, $err:expr) => {{
        if $f.is_null() {
            set_error("null factory handle");
            return $err;
        }
        &(*$f).inner
    }};
}

/// Append hosts from a CIDR subnet. Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_with_subnet(
    factory: *mut AsicFactory,
    subnet: *const c_char,
) -> c_int {
    clear_error();
    let f = factory_mut_ret!(factory, -1);
    let subnet = match cstr_to_str(subnet) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    match f.clone().with_subnet(subnet) {
        Ok(updated) => {
            *f = updated;
            0
        }
        Err(e) => {
            set_error_from(e);
            -1
        }
    }
}

/// Append hosts from a range string. Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_with_range(
    factory: *mut AsicFactory,
    range: *const c_char,
) -> c_int {
    clear_error();
    let f = factory_mut_ret!(factory, -1);
    let range = match cstr_to_str(range) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    match f.clone().with_range(range) {
        Ok(updated) => {
            *f = updated;
            0
        }
        Err(e) => {
            set_error_from(e);
            -1
        }
    }
}

/// Append hosts from octet ranges. Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_with_octets(
    factory: *mut AsicFactory,
    o1: *const c_char,
    o2: *const c_char,
    o3: *const c_char,
    o4: *const c_char,
) -> c_int {
    clear_error();
    let f = factory_mut_ret!(factory, -1);
    let parse = |p| match cstr_to_str(p) {
        Ok(s) => Ok(s),
        Err(e) => Err(e),
    };
    let (a, b, c, d) = match (parse(o1), parse(o2), parse(o3), parse(o4)) {
        (Ok(a), Ok(b), Ok(c), Ok(d)) => (a, b, c, d),
        (Err(e), _, _, _) | (_, Err(e), _, _) | (_, _, Err(e), _) | (_, _, _, Err(e)) => {
            set_error(e);
            return -1;
        }
    };
    match f.clone().with_octets(a, b, c, d) {
        Ok(updated) => {
            *f = updated;
            0
        }
        Err(e) => {
            set_error_from(e);
            -1
        }
    }
}

/// Enable or disable the initial port connectivity check.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_port_check(factory: *mut AsicFactory, enabled: bool) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_port_check(enabled);
}

/// Set concurrent discovery limit.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_concurrent_limit(
    factory: *mut AsicFactory,
    limit: usize,
) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_concurrent_limit(limit);
}

/// Set identification timeout in seconds.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_identification_timeout_secs(
    factory: *mut AsicFactory,
    secs: u64,
) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_identification_timeout_secs(secs);
}

/// Set connectivity timeout in seconds.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_connectivity_timeout_secs(
    factory: *mut AsicFactory,
    secs: u64,
) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_connectivity_timeout_secs(secs);
}

/// Set connectivity retries.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_connectivity_retries(
    factory: *mut AsicFactory,
    retries: u32,
) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_connectivity_retries(retries);
}

/// Set nofile (RLIMIT_NOFILE) target for large scans.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_nofile_limit(factory: *mut AsicFactory, limit: u64) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_nofile_limit(limit);
}

/// Enable or disable automatic nofile adjustment.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_nofile_adjustment(
    factory: *mut AsicFactory,
    enabled: bool,
) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_nofile_adjustment(enabled);
}

/// Apply adaptive concurrency based on the current host list size.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_set_adaptive_concurrency(factory: *mut AsicFactory) {
    clear_error();
    let f = factory_mut!(factory);
    *f = f.clone().with_adaptive_concurrency();
}

/// Number of hosts currently configured for scanning.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_len(factory: *const AsicFactory) -> c_int {
    clear_error();
    if factory.is_null() {
        set_error("null factory handle");
        return -1;
    }
    (*factory).inner.len() as c_int
}

/// Whether the factory has no hosts configured.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_is_empty(factory: *const AsicFactory) -> bool {
    clear_error();
    if factory.is_null() {
        set_error("null factory handle");
        return true;
    }
    (*factory).inner.is_empty()
}

/// JSON array of configured host IP strings. Free with [`asic_rs_free_string`].
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_hosts_json(factory: *const AsicFactory) -> *mut c_char {
    clear_error();
    let f = factory_ref!(factory, ptr::null_mut());
    let hosts: Vec<String> = f.hosts().iter().map(|ip| ip.to_string()).collect();
    json_to_c_string(&hosts)
}

/// Discover and construct a miner at `ip`. Returns null if not found / error.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_get_miner(
    factory: *mut AsicFactory,
    ip: *const c_char,
) -> *mut AsicMiner {
    clear_error();
    let f = factory_ref!(factory, ptr::null_mut());
    let ip_str = match cstr_to_str(ip) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return ptr::null_mut();
        }
    };
    let ip_addr: IpAddr = match ip_str.parse() {
        Ok(ip) => ip,
        Err(e) => {
            set_error_from(e);
            return ptr::null_mut();
        }
    };
    match RUNTIME.block_on(f.get_miner(ip_addr)) {
        Ok(Some(miner)) => Box::into_raw(Box::new(AsicMiner { inner: miner })),
        Ok(None) => {
            set_error(format!("no supported miner found at {ip_str}"));
            ptr::null_mut()
        }
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

/// Scan a single IP with port pre-check (like scan_miner). Returns null if none.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_scan_miner(
    factory: *mut AsicFactory,
    ip: *const c_char,
) -> *mut AsicMiner {
    clear_error();
    let f = factory_ref!(factory, ptr::null_mut());
    let ip_str = match cstr_to_str(ip) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return ptr::null_mut();
        }
    };
    let ip_addr: IpAddr = match ip_str.parse() {
        Ok(ip) => ip,
        Err(e) => {
            set_error_from(e);
            return ptr::null_mut();
        }
    };
    match RUNTIME.block_on(f.scan_miner(ip_addr)) {
        Ok(Some(miner)) => Box::into_raw(Box::new(AsicMiner { inner: miner })),
        Ok(None) => {
            set_error(format!("no miner responded at {ip_str}"));
            ptr::null_mut()
        }
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

/// Scan all configured hosts. Returns a JSON array of miner summary objects
/// plus an out-list of miner handles via `out_miners` / `out_len`.
///
/// On success: allocates `*out_miners` as an array of `*mut AsicMiner` of length
/// `*out_len`. Free each miner with [`asic_rs_miner_free`], then free the array
/// with [`asic_rs_free_miner_list`].
///
/// Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_factory_scan(
    factory: *mut AsicFactory,
    out_miners: *mut *mut *mut AsicMiner,
    out_len: *mut usize,
) -> c_int {
    clear_error();
    if out_miners.is_null() || out_len.is_null() {
        set_error("null out_miners or out_len");
        return -1;
    }
    let f = factory_ref!(factory, -1);
    match RUNTIME.block_on(f.scan()) {
        Ok(miners) => {
            let len = miners.len();
            let mut handles: Vec<*mut AsicMiner> = miners
                .into_iter()
                .map(|m| Box::into_raw(Box::new(AsicMiner { inner: m })))
                .collect();
            let ptr = handles.as_mut_ptr();
            std::mem::forget(handles);
            *out_miners = ptr;
            *out_len = len;
            0
        }
        Err(e) => {
            set_error_from(e);
            -1
        }
    }
}

/// Free an array of miner pointers returned by [`asic_rs_factory_scan`].
/// Does **not** free the individual miners — call [`asic_rs_miner_free`] first.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_free_miner_list(list: *mut *mut AsicMiner, len: usize) {
    if list.is_null() {
        return;
    }
    drop(Vec::from_raw_parts(list, len, len));
}

// ---------------------------------------------------------------------------
// Miner identity / capabilities
// ---------------------------------------------------------------------------

macro_rules! miner_ref {
    ($m:expr, $err:expr) => {{
        if $m.is_null() {
            set_error("null miner handle");
            return $err;
        }
        &(*$m).inner
    }};
}

macro_rules! miner_mut {
    ($m:expr, $err:expr) => {{
        if $m.is_null() {
            set_error("null miner handle");
            return $err;
        }
        &mut (*$m).inner
    }};
}

/// Free a miner handle.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_free(miner: *mut AsicMiner) {
    if !miner.is_null() {
        drop(Box::from_raw(miner));
    }
}

/// IP address as a newly allocated C string. Free with [`asic_rs_free_string`].
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_ip(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    to_c_string(m.get_ip().to_string())
}

/// Device info as JSON. Free with [`asic_rs_free_string`].
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_device_info_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    json_to_c_string(&m.get_device_info())
}

/// Human-readable summary: "Make Model (Firmware): IP". Free with free_string.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_summary(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let info = m.get_device_info();
    to_c_string(format!(
        "{} {} ({}): {}",
        info.make,
        info.model,
        info.firmware,
        m.get_ip()
    ))
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_set_fault_light(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_set_fault_light()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_set_power_limit(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_set_power_limit()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_restart(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_restart()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_pause(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_pause()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_resume(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_resume()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_pools_config(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_pools_config()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_upgrade_firmware(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_upgrade_firmware()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_scaling_config(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_scaling_config()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_tuning_config(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_tuning_config()
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_supports_fan_config(miner: *const AsicMiner) -> bool {
    clear_error();
    miner_ref!(miner, false).supports_fan_config()
}

/// Set credentials for subsequent privileged operations.
/// Returns 0 on success, -1 on error.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_set_auth(
    miner: *mut AsicMiner,
    username: *const c_char,
    password: *const c_char,
) -> c_int {
    clear_error();
    let m = miner_mut!(miner, -1);
    let user = match cstr_to_str(username) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    let pass = match cstr_to_str(password) {
        Ok(s) => s,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    m.set_auth(MinerAuth::new(user, pass));
    0
}

// ---------------------------------------------------------------------------
// Data collection
// ---------------------------------------------------------------------------

/// Full MinerData as JSON. Free with [`asic_rs_free_string`].
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_data_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let data = RUNTIME.block_on(m.get_data());
    json_to_c_string(&data)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_mac_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_mac()).map(|mac| mac.to_string());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_serial_number_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_serial_number());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_hostname_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_hostname());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_api_version_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_api_version());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_firmware_version_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_firmware_version());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_control_board_version_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME
        .block_on(m.get_control_board_version())
        .map(|cb| cb.to_string());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_hashboards_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_hashboards());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_hashrate_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_hashrate());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_expected_hashrate_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_expected_hashrate());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_fans_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_fans());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_psu_fans_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_psu_fans());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_fluid_temperature_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME
        .block_on(m.get_fluid_temperature())
        .map(|t| t.as_celsius());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_wattage_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_wattage()).map(|w| w.as_watts());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_tuning_target_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_tuning_target());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_light_flashing_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_light_flashing());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_messages_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_messages());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_uptime_secs_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_uptime()).map(|d| d.as_secs());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_is_mining_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_is_mining());
    json_to_c_string(&value)
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_pools_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    let value = RUNTIME.block_on(m.get_pools());
    json_to_c_string(&value)
}

// ---------------------------------------------------------------------------
// Config get/set (JSON in / JSON out)
// ---------------------------------------------------------------------------

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_pools_config_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    match RUNTIME.block_on(m.get_pools_config()) {
        Ok(cfg) => json_to_c_string(&cfg),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_scaling_config_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    match RUNTIME.block_on(m.get_scaling_config()) {
        Ok(cfg) => json_to_c_string(&cfg),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_tuning_config_json(
    miner: *const AsicMiner,
) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    match RUNTIME.block_on(m.get_tuning_config()) {
        Ok(cfg) => json_to_c_string(&cfg),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_get_fan_config_json(miner: *const AsicMiner) -> *mut c_char {
    clear_error();
    let m = miner_ref!(miner, ptr::null_mut());
    match RUNTIME.block_on(m.get_fan_config()) {
        Ok(cfg) => json_to_c_string(&cfg),
        Err(e) => {
            set_error_from(e);
            ptr::null_mut()
        }
    }
}

/// Returns 1 if accepted, 0 if rejected/unsupported, -1 on error.
unsafe fn control_bool_result(
    miner: *const AsicMiner,
    fut: impl std::future::Future<Output = anyhow::Result<bool>>,
) -> c_int {
    clear_error();
    if miner.is_null() {
        set_error("null miner handle");
        return -1;
    }
    match RUNTIME.block_on(fut) {
        Ok(true) => 1,
        Ok(false) => 0,
        Err(e) => {
            set_error_from(e);
            -1
        }
    }
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_set_pools_config_json(
    miner: *const AsicMiner,
    json: *const c_char,
) -> c_int {
    let cfg: Vec<PoolGroupConfig> = match parse_json(json) {
        Ok(v) => v,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    let m = miner_ref!(miner, -1);
    control_bool_result(miner, m.set_pools_config(cfg))
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_set_scaling_config_json(
    miner: *const AsicMiner,
    json: *const c_char,
) -> c_int {
    let cfg: ScalingConfig = match parse_json(json) {
        Ok(v) => v,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    let m = miner_ref!(miner, -1);
    control_bool_result(miner, m.set_scaling_config(cfg))
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_set_tuning_config_json(
    miner: *const AsicMiner,
    config_json: *const c_char,
    scaling_json: *const c_char,
) -> c_int {
    let cfg: TuningConfig = match parse_json(config_json) {
        Ok(v) => v,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    let scaling: Option<ScalingConfig> = if scaling_json.is_null() {
        None
    } else {
        match parse_json(scaling_json) {
            Ok(v) => Some(v),
            Err(e) => {
                set_error(e);
                return -1;
            }
        }
    };
    let m = miner_ref!(miner, -1);
    control_bool_result(miner, m.set_tuning_config(cfg, scaling))
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_set_fan_config_json(
    miner: *const AsicMiner,
    json: *const c_char,
) -> c_int {
    let cfg: FanConfig = match parse_json(json) {
        Ok(v) => v,
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    let m = miner_ref!(miner, -1);
    control_bool_result(miner, m.set_fan_config(cfg))
}

// ---------------------------------------------------------------------------
// Control
// ---------------------------------------------------------------------------

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_set_fault_light(
    miner: *const AsicMiner,
    fault: bool,
) -> c_int {
    let m = miner_ref!(miner, -1);
    control_bool_result(miner, m.set_fault_light(fault))
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_set_power_limit(
    miner: *const AsicMiner,
    watts: f64,
) -> c_int {
    let m = miner_ref!(miner, -1);
    control_bool_result(miner, m.set_power_limit(Power::from_watts(watts)))
}

#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_restart(miner: *const AsicMiner) -> c_int {
    let m = miner_ref!(miner, -1);
    control_bool_result(miner, m.restart())
}

/// Pause mining. `at_time_secs` < 0 means "now" (None).
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_pause(miner: *const AsicMiner, at_time_secs: f64) -> c_int {
    let m = miner_ref!(miner, -1);
    let at = if at_time_secs < 0.0 {
        None
    } else {
        Some(Duration::from_secs_f64(at_time_secs))
    };
    control_bool_result(miner, m.pause(at))
}

/// Resume mining. `at_time_secs` < 0 means "now" (None).
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_resume(miner: *const AsicMiner, at_time_secs: f64) -> c_int {
    let m = miner_ref!(miner, -1);
    let at = if at_time_secs < 0.0 {
        None
    } else {
        Some(Duration::from_secs_f64(at_time_secs))
    };
    control_bool_result(miner, m.resume(at))
}

/// Upgrade firmware from a local file path. Returns 1/0/-1.
#[no_mangle]
pub unsafe extern "C" fn asic_rs_miner_upgrade_firmware(
    miner: *const AsicMiner,
    path: *const c_char,
) -> c_int {
    clear_error();
    let m = miner_ref!(miner, -1);
    let path = match cstr_to_str(path) {
        Ok(s) => s.to_string(),
        Err(e) => {
            set_error(e);
            return -1;
        }
    };
    match RUNTIME.block_on(async {
        let image = FirmwareImage::from_file_async(&path).await?;
        m.upgrade_firmware(image).await
    }) {
        Ok(true) => 1,
        Ok(false) => 0,
        Err(e) => {
            set_error_from(e);
            -1
        }
    }
}
