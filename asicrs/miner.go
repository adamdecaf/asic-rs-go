package asicrs

/*
#include "asic_rs_ffi.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"
)

// Miner is a handle to a discovered ASIC miner.
//
// Always call Close when finished (or use defer). Methods are safe for
// concurrent use from multiple goroutines only if the underlying asic-rs
// backend allows it; prefer one goroutine per miner for control operations.
type Miner struct {
	ptr *C.AsicMiner
}

func newMiner(ptr *C.AsicMiner) *Miner {
	m := &Miner{ptr: ptr}
	runtime.SetFinalizer(m, (*Miner).Close)
	return m
}

// Close frees the miner handle. Safe to call multiple times.
func (m *Miner) Close() {
	if m == nil || m.ptr == nil {
		return
	}
	C.asic_rs_miner_free(m.ptr)
	m.ptr = nil
	runtime.SetFinalizer(m, nil)
}

func (m *Miner) check() error {
	if m == nil || m.ptr == nil {
		return fmt.Errorf("miner is closed or nil")
	}
	return nil
}

// IP returns the miner's IP address.
func (m *Miner) IP() (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	s := C.asic_rs_miner_ip(m.ptr)
	if s == nil {
		return "", lastError()
	}
	return goStringOwned(s), nil
}

// DeviceInfo returns static make/model/firmware information.
func (m *Miner) DeviceInfo() (DeviceInfo, error) {
	var info DeviceInfo
	if err := m.check(); err != nil {
		return info, err
	}
	if err := takeJSON(C.asic_rs_miner_device_info_json(m.ptr), &info); err != nil {
		return info, err
	}
	return info, nil
}

// Summary returns a short human-readable description.
func (m *Miner) Summary() (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	s := C.asic_rs_miner_summary(m.ptr)
	if s == nil {
		return "", lastError()
	}
	return goStringOwned(s), nil
}

// Supports reports which control/config features this miner backend exposes.
type Supports struct {
	SetFaultLight    bool
	SetPowerLimit    bool
	Restart          bool
	Pause            bool
	Resume           bool
	PoolsConfig      bool
	UpgradeFirmware  bool
	ScalingConfig    bool
	TuningConfig     bool
	FanConfig        bool
}

// Supports returns capability flags for this miner.
func (m *Miner) Supports() (Supports, error) {
	var s Supports
	if err := m.check(); err != nil {
		return s, err
	}
	s.SetFaultLight = bool(C.asic_rs_miner_supports_set_fault_light(m.ptr))
	s.SetPowerLimit = bool(C.asic_rs_miner_supports_set_power_limit(m.ptr))
	s.Restart = bool(C.asic_rs_miner_supports_restart(m.ptr))
	s.Pause = bool(C.asic_rs_miner_supports_pause(m.ptr))
	s.Resume = bool(C.asic_rs_miner_supports_resume(m.ptr))
	s.PoolsConfig = bool(C.asic_rs_miner_supports_pools_config(m.ptr))
	s.UpgradeFirmware = bool(C.asic_rs_miner_supports_upgrade_firmware(m.ptr))
	s.ScalingConfig = bool(C.asic_rs_miner_supports_scaling_config(m.ptr))
	s.TuningConfig = bool(C.asic_rs_miner_supports_tuning_config(m.ptr))
	s.FanConfig = bool(C.asic_rs_miner_supports_fan_config(m.ptr))
	return s, nil
}

// SetAuth sets credentials for subsequent privileged operations.
func (m *Miner) SetAuth(username, password string) error {
	if err := m.check(); err != nil {
		return err
	}
	cu, cp := cString(username), cString(password)
	defer freeGoCString(cu)
	defer freeGoCString(cp)
	if C.asic_rs_miner_set_auth(m.ptr, cu, cp) != 0 {
		return lastError()
	}
	return nil
}

// GetData collects a full MinerData snapshot.
func (m *Miner) GetData() (*MinerData, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var data MinerData
	if err := takeJSON(C.asic_rs_miner_get_data_json(m.ptr), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetDataJSON returns the raw MinerData JSON bytes without unmarshaling.
func (m *Miner) GetDataJSON() ([]byte, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return takeJSONBytes(C.asic_rs_miner_get_data_json(m.ptr))
}

// GetMAC returns the miner MAC address string, if available.
func (m *Miner) GetMAC() (*string, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *string
	return v, takeJSON(C.asic_rs_miner_get_mac_json(m.ptr), &v)
}

// GetSerialNumber returns the control-board serial, if available.
func (m *Miner) GetSerialNumber() (*string, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *string
	return v, takeJSON(C.asic_rs_miner_get_serial_number_json(m.ptr), &v)
}

// GetHostname returns the network hostname, if available.
func (m *Miner) GetHostname() (*string, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *string
	return v, takeJSON(C.asic_rs_miner_get_hostname_json(m.ptr), &v)
}

// GetAPIVersion returns the device API version string, if available.
func (m *Miner) GetAPIVersion() (*string, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *string
	return v, takeJSON(C.asic_rs_miner_get_api_version_json(m.ptr), &v)
}

// GetFirmwareVersion returns the firmware version string, if available.
func (m *Miner) GetFirmwareVersion() (*string, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *string
	return v, takeJSON(C.asic_rs_miner_get_firmware_version_json(m.ptr), &v)
}

// GetControlBoardVersion returns the control board type string, if available.
func (m *Miner) GetControlBoardVersion() (*string, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *string
	return v, takeJSON(C.asic_rs_miner_get_control_board_version_json(m.ptr), &v)
}

// GetHashboards returns per-board telemetry.
func (m *Miner) GetHashboards() ([]BoardData, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v []BoardData
	return v, takeJSON(C.asic_rs_miner_get_hashboards_json(m.ptr), &v)
}

// GetHashrate returns the current hashrate, if available.
func (m *Miner) GetHashrate() (*HashRate, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *HashRate
	return v, takeJSON(C.asic_rs_miner_get_hashrate_json(m.ptr), &v)
}

// GetExpectedHashrate returns the expected/factory hashrate, if available.
func (m *Miner) GetExpectedHashrate() (*HashRate, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *HashRate
	return v, takeJSON(C.asic_rs_miner_get_expected_hashrate_json(m.ptr), &v)
}

// GetFans returns chassis fan readings.
func (m *Miner) GetFans() ([]FanData, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v []FanData
	return v, takeJSON(C.asic_rs_miner_get_fans_json(m.ptr), &v)
}

// GetPSUFans returns PSU fan readings.
func (m *Miner) GetPSUFans() ([]FanData, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v []FanData
	return v, takeJSON(C.asic_rs_miner_get_psu_fans_json(m.ptr), &v)
}

// GetFluidTemperature returns environment/fluid temperature in °C, if available.
func (m *Miner) GetFluidTemperature() (*float64, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *float64
	return v, takeJSON(C.asic_rs_miner_get_fluid_temperature_json(m.ptr), &v)
}

// GetWattage returns power draw in watts, if available.
func (m *Miner) GetWattage() (*float64, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *float64
	return v, takeJSON(C.asic_rs_miner_get_wattage_json(m.ptr), &v)
}

// GetTuningTarget returns the current tuning target, if available.
func (m *Miner) GetTuningTarget() (*TuningTarget, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *TuningTarget
	return v, takeJSON(C.asic_rs_miner_get_tuning_target_json(m.ptr), &v)
}

// GetLightFlashing returns the fault-light state, if available.
func (m *Miner) GetLightFlashing() (*bool, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v *bool
	return v, takeJSON(C.asic_rs_miner_get_light_flashing_json(m.ptr), &v)
}

// GetMessages returns device status/error messages.
func (m *Miner) GetMessages() ([]MinerMessage, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v []MinerMessage
	return v, takeJSON(C.asic_rs_miner_get_messages_json(m.ptr), &v)
}

// GetUptime returns system uptime, if available.
func (m *Miner) GetUptime() (*time.Duration, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var secs *uint64
	if err := takeJSON(C.asic_rs_miner_get_uptime_secs_json(m.ptr), &secs); err != nil {
		return nil, err
	}
	if secs == nil {
		return nil, nil
	}
	d := time.Duration(*secs) * time.Second
	return &d, nil
}

// GetIsMining reports whether hashing is currently running.
func (m *Miner) GetIsMining() (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	var v bool
	if err := takeJSON(C.asic_rs_miner_get_is_mining_json(m.ptr), &v); err != nil {
		return false, err
	}
	return v, nil
}

// GetPools returns configured pool groups with runtime stats.
func (m *Miner) GetPools() ([]PoolGroupData, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	var v []PoolGroupData
	return v, takeJSON(C.asic_rs_miner_get_pools_json(m.ptr), &v)
}

// GetPoolsConfig returns the pools configuration (writable form).
func (m *Miner) GetPoolsConfig() ([]PoolGroupConfig, error) {
	var v []PoolGroupConfig
	if err := m.check(); err != nil {
		return nil, err
	}
	if err := takeJSON(C.asic_rs_miner_get_pools_config_json(m.ptr), &v); err != nil {
		return nil, err
	}
	return v, nil
}

// GetScalingConfig returns the scaling configuration.
func (m *Miner) GetScalingConfig() (*ScalingConfig, error) {
	var v ScalingConfig
	if err := m.check(); err != nil {
		return nil, err
	}
	if err := takeJSON(C.asic_rs_miner_get_scaling_config_json(m.ptr), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// GetTuningConfig returns the tuning configuration.
func (m *Miner) GetTuningConfig() (*TuningConfig, error) {
	var v TuningConfig
	if err := m.check(); err != nil {
		return nil, err
	}
	if err := takeJSON(C.asic_rs_miner_get_tuning_config_json(m.ptr), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// GetFanConfig returns the fan configuration.
func (m *Miner) GetFanConfig() (*FanConfig, error) {
	var v FanConfig
	if err := m.check(); err != nil {
		return nil, err
	}
	if err := takeJSON(C.asic_rs_miner_get_fan_config_json(m.ptr), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (m *Miner) marshalConfig(v any) (*C.char, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return cString(string(b)), nil
}

// SetPoolsConfig applies a pools configuration.
func (m *Miner) SetPoolsConfig(groups []PoolGroupConfig) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	cs, err := m.marshalConfig(groups)
	if err != nil {
		return false, err
	}
	defer freeGoCString(cs)
	return controlResult(C.asic_rs_miner_set_pools_config_json(m.ptr, cs))
}

// SetScalingConfig applies a scaling configuration.
func (m *Miner) SetScalingConfig(cfg ScalingConfig) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	cs, err := m.marshalConfig(cfg)
	if err != nil {
		return false, err
	}
	defer freeGoCString(cs)
	return controlResult(C.asic_rs_miner_set_scaling_config_json(m.ptr, cs))
}

// SetTuningConfig applies a tuning configuration, with optional scaling.
func (m *Miner) SetTuningConfig(cfg TuningConfig, scaling *ScalingConfig) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return false, err
	}
	cc := cString(string(cfgJSON))
	defer freeGoCString(cc)
	var sc *C.char
	if scaling != nil {
		sb, err := json.Marshal(scaling)
		if err != nil {
			return false, err
		}
		sc = cString(string(sb))
		defer freeGoCString(sc)
	}
	return controlResult(C.asic_rs_miner_set_tuning_config_json(m.ptr, cc, sc))
}

// SetFanConfig applies a fan configuration.
func (m *Miner) SetFanConfig(cfg FanConfig) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	cs, err := m.marshalConfig(cfg)
	if err != nil {
		return false, err
	}
	defer freeGoCString(cs)
	return controlResult(C.asic_rs_miner_set_fan_config_json(m.ptr, cs))
}

// SetFaultLight turns the fault/alert light on or off.
func (m *Miner) SetFaultLight(fault bool) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	return controlResult(C.asic_rs_miner_set_fault_light(m.ptr, C.bool(fault)))
}

// SetPowerLimit sets a power limit in watts.
func (m *Miner) SetPowerLimit(watts float64) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	return controlResult(C.asic_rs_miner_set_power_limit(m.ptr, C.double(watts)))
}

// Restart reboots the miner.
func (m *Miner) Restart() (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	return controlResult(C.asic_rs_miner_restart(m.ptr))
}

// Pause pauses mining. A nil atTime means immediately.
func (m *Miner) Pause(atTime *time.Duration) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	secs := -1.0
	if atTime != nil {
		secs = atTime.Seconds()
	}
	return controlResult(C.asic_rs_miner_pause(m.ptr, C.double(secs)))
}

// Resume resumes mining. A nil atTime means immediately.
func (m *Miner) Resume(atTime *time.Duration) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	secs := -1.0
	if atTime != nil {
		secs = atTime.Seconds()
	}
	return controlResult(C.asic_rs_miner_resume(m.ptr, C.double(secs)))
}

// UpgradeFirmware uploads and applies a firmware image from a local file path.
func (m *Miner) UpgradeFirmware(path string) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	cs := cString(path)
	defer freeGoCString(cs)
	return controlResult(C.asic_rs_miner_upgrade_firmware(m.ptr, cs))
}
