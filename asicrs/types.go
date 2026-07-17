package asicrs

import (
	"encoding/json"
	"fmt"
	"time"
)

// HashRate is a hashrate value with unit and algorithm.
type HashRate struct {
	Value float64      `json:"value"`
	Unit  HashRateUnit `json:"unit"`
	Algo  string       `json:"algo"`
}

// HashRateUnit is the scale of a HashRate.Value.
type HashRateUnit string

const (
	HashRateUnitHash      HashRateUnit = "Hash"
	HashRateUnitKiloHash  HashRateUnit = "KiloHash"
	HashRateUnitMegaHash  HashRateUnit = "MegaHash"
	HashRateUnitGigaHash  HashRateUnit = "GigaHash"
	HashRateUnitTeraHash  HashRateUnit = "TeraHash"
	HashRateUnitPetaHash  HashRateUnit = "PetaHash"
	HashRateUnitExaHash   HashRateUnit = "ExaHash"
	HashRateUnitZettaHash HashRateUnit = "ZettaHash"
	HashRateUnitYottaHash HashRateUnit = "YottaHash"
)

// Multiplier returns the factor to convert this unit to H/s.
func (u HashRateUnit) Multiplier() float64 {
	switch u {
	case HashRateUnitHash:
		return 1
	case HashRateUnitKiloHash:
		return 1e3
	case HashRateUnitMegaHash:
		return 1e6
	case HashRateUnitGigaHash:
		return 1e9
	case HashRateUnitTeraHash:
		return 1e12
	case HashRateUnitPetaHash:
		return 1e15
	case HashRateUnitExaHash:
		return 1e18
	case HashRateUnitZettaHash:
		return 1e21
	case HashRateUnitYottaHash:
		return 1e24
	default:
		return 1
	}
}

// AsUnit converts hr into the requested unit.
func (hr HashRate) AsUnit(unit HashRateUnit) HashRate {
	base := hr.Value * hr.Unit.Multiplier()
	return HashRate{
		Value: base / unit.Multiplier(),
		Unit:  unit,
		Algo:  hr.Algo,
	}
}

// TH returns the hashrate in TH/s.
func (hr HashRate) TH() float64 {
	return hr.AsUnit(HashRateUnitTeraHash).Value
}

// DeviceInfo is static identity information for a miner.
type DeviceInfo struct {
	Make     string        `json:"make"`
	Model    string        `json:"model"`
	Hardware MinerHardware `json:"hardware"`
	Firmware string        `json:"firmware"`
	Algo     string        `json:"algo"`
}

// MinerHardware describes expected fans/boards/chips.
//
// In asic-rs 0.7, boards is a slice of per-board expected chip counts
// (null entries allowed). Older layouts used a single board count.
type MinerHardware struct {
	Fans   *uint8  `json:"fans"`
	Boards any     `json:"boards"` // []optional chip counts or legacy number
	Chips  *uint16 `json:"chips,omitempty"`
}

// BoardCount returns the expected number of hashboards when available.
func (h MinerHardware) BoardCount() (int, bool) {
	switch v := h.Boards.(type) {
	case nil:
		return 0, false
	case float64:
		return int(v), true
	case []any:
		return len(v), true
	default:
		return 0, false
	}
}

// ChipData is optional per-chip telemetry.
type ChipData struct {
	Position    *int      `json:"position"`
	Hashrate    *HashRate `json:"hashrate"`
	Temperature *float64  `json:"temperature"`
	Voltage     *float64  `json:"voltage"`
	Frequency   *float64  `json:"frequency"`
	Working     *bool     `json:"working"`
	Tuned       *bool     `json:"tuned"`
}

// BoardData is per-hashboard telemetry.
type BoardData struct {
	Position              uint8      `json:"position"`
	Hashrate              *HashRate  `json:"hashrate"`
	ExpectedHashrate      *HashRate  `json:"expected_hashrate"`
	BoardTemperature      *float64   `json:"board_temperature"`
	InletChipTemperature  *float64   `json:"inlet_chip_temperature"`
	OutletChipTemperature *float64   `json:"outlet_chip_temperature"`
	// Legacy / alternate names some firmwares may surface via JSON aliases.
	IntakeTemperature *float64   `json:"intake_temperature"`
	OutletTemperature *float64   `json:"outlet_temperature"`
	ExpectedChips     *uint16    `json:"expected_chips"`
	WorkingChips      *uint16    `json:"working_chips"`
	SerialNumber      *string    `json:"serial_number"`
	Chips             []ChipData `json:"chips"`
	Voltage           *float64   `json:"voltage"`
	Frequency         *float64   `json:"frequency"`
	Tuned             *bool      `json:"tuned"`
	Active            *bool      `json:"active"`
}

// FanData is a single fan reading (RPM as float when present).
type FanData struct {
	Position int16    `json:"position"`
	RPM      *float64 `json:"rpm"`
}

// MessageSeverity is the severity of a miner message.
type MessageSeverity string

// MinerMessage is a status/error message from the device.
type MinerMessage struct {
	Timestamp uint32          `json:"timestamp"`
	Code      uint64          `json:"code"`
	Message   string          `json:"message"`
	Severity  MessageSeverity `json:"severity"`
	Component json.RawMessage `json:"component,omitempty"`
}

// PoolScheme is the stratum protocol scheme.
type PoolScheme string

// PoolURL is a parsed mining pool endpoint.
type PoolURL struct {
	Scheme PoolScheme `json:"scheme"`
	Host   string     `json:"host"`
	Port   uint16     `json:"port"`
	Pubkey *string    `json:"pubkey"`
}

// String formats the pool URL.
func (u PoolURL) String() string {
	if u.Pubkey != nil && *u.Pubkey != "" {
		return fmt.Sprintf("%s://%s:%d/%s", u.Scheme, u.Host, u.Port, *u.Pubkey)
	}
	return fmt.Sprintf("%s://%s:%d", u.Scheme, u.Host, u.Port)
}

// PoolData is runtime status for one configured pool.
type PoolData struct {
	Position       *uint16  `json:"position"`
	URL            *PoolURL `json:"url"`
	AcceptedShares *uint64  `json:"accepted_shares"`
	RejectedShares *uint64  `json:"rejected_shares"`
	Active         *bool    `json:"active"`
	Alive          *bool    `json:"alive"`
	User           *string  `json:"user"`
}

// PoolGroupData is a group of pools with a quota.
type PoolGroupData struct {
	Name  string     `json:"name"`
	Quota uint32     `json:"quota"`
	Pools []PoolData `json:"pools"`
}

// DurationSecs unmarshals a Rust std::time::Duration ({secs,nanos}) or a bare number of seconds.
type DurationSecs struct {
	Secs  uint64 `json:"secs"`
	Nanos uint32 `json:"nanos"`
}

// Duration converts to time.Duration.
func (d DurationSecs) Duration() time.Duration {
	return time.Duration(d.Secs)*time.Second + time.Duration(d.Nanos)*time.Nanosecond
}

// UnmarshalJSON accepts {"secs":N,"nanos":M} or a numeric seconds value.
func (d *DurationSecs) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var n uint64
	if err := json.Unmarshal(b, &n); err == nil {
		d.Secs = n
		d.Nanos = 0
		return nil
	}
	type raw DurationSecs
	return json.Unmarshal(b, (*raw)(d))
}

// PowerWatts unmarshals measurements::Power which serializes as {"watts": N},
// or a bare number (when asic-rs custom serializers are used).
type PowerWatts float64

// UnmarshalJSON accepts {"watts":N} or a bare number.
func (p *PowerWatts) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*p = PowerWatts(n)
		return nil
	}
	var obj struct {
		Watts float64 `json:"watts"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	*p = PowerWatts(obj.Watts)
	return nil
}

// Float64 returns the wattage.
func (p PowerWatts) Float64() float64 { return float64(p) }

// TuningTarget is a firmware tuning target. The Rust enum is externally tagged:
//
//	{"Power":{"watts":3500}} | {"HashRate":{...}} | {"MiningMode":"Normal"}
type TuningTarget struct {
	Kind     string    // "Power", "HashRate", or "MiningMode"
	Watts    *float64  `json:"-"`
	HashRate *HashRate `json:"-"`
	Mode     *string   `json:"-"`
	Raw      json.RawMessage
}

// UnmarshalJSON decodes the externally-tagged TuningTarget enum.
func (t *TuningTarget) UnmarshalJSON(b []byte) error {
	t.Raw = append(t.Raw[:0], b...)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		// bare string mode fallback
		var s string
		if err2 := json.Unmarshal(b, &s); err2 == nil {
			t.Kind = "MiningMode"
			t.Mode = &s
			return nil
		}
		return err
	}
	if raw, ok := m["Power"]; ok {
		t.Kind = "Power"
		var pw PowerWatts
		if err := json.Unmarshal(raw, &pw); err != nil {
			return err
		}
		w := pw.Float64()
		t.Watts = &w
		return nil
	}
	if raw, ok := m["HashRate"]; ok {
		t.Kind = "HashRate"
		var hr HashRate
		if err := json.Unmarshal(raw, &hr); err != nil {
			return err
		}
		t.HashRate = &hr
		return nil
	}
	if raw, ok := m["MiningMode"]; ok {
		t.Kind = "MiningMode"
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		t.Mode = &s
		return nil
	}
	return fmt.Errorf("unknown TuningTarget variant: %s", string(b))
}

// MarshalJSON encodes TuningTarget in the Rust externally-tagged form.
func (t TuningTarget) MarshalJSON() ([]byte, error) {
	switch t.Kind {
	case "Power":
		w := 0.0
		if t.Watts != nil {
			w = *t.Watts
		}
		return json.Marshal(map[string]any{"Power": map[string]float64{"watts": w}})
	case "HashRate":
		return json.Marshal(map[string]any{"HashRate": t.HashRate})
	case "MiningMode":
		mode := ""
		if t.Mode != nil {
			mode = *t.Mode
		}
		return json.Marshal(map[string]any{"MiningMode": mode})
	default:
		if len(t.Raw) > 0 {
			return t.Raw, nil
		}
		return nil, fmt.Errorf("empty TuningTarget")
	}
}

// MinerData is a full telemetry snapshot from a miner.
type MinerData struct {
	SchemaVersion          string          `json:"schema_version"`
	Timestamp              uint64          `json:"timestamp"`
	IP                     string          `json:"ip"`
	MAC                    *string         `json:"mac"`
	DeviceInfo             DeviceInfo      `json:"device_info"`
	SerialNumber           *string         `json:"serial_number"`
	Hostname               *string         `json:"hostname"`
	APIVersion             *string         `json:"api_version"`
	FirmwareVersion        *string         `json:"firmware_version"`
	ControlBoardVersion    json.RawMessage `json:"control_board_version"`
	ExpectedHashboards     *uint8          `json:"expected_hashboards"`
	Hashboards             []BoardData     `json:"hashboards"`
	Hashrate               *HashRate       `json:"hashrate"`
	ExpectedHashrate       *HashRate       `json:"expected_hashrate"`
	ExpectedChips          *uint16         `json:"expected_chips"`
	TotalChips             *uint16         `json:"total_chips"`
	ExpectedFans           *uint8          `json:"expected_fans"`
	Fans                   []FanData       `json:"fans"`
	PSUFans                []FanData       `json:"psu_fans"`
	AverageTemperature     *float64        `json:"average_temperature"`
	FluidTemperature       *float64        `json:"fluid_temperature"`
	OutletFluidTemperature *float64        `json:"outlet_fluid_temperature"`
	Wattage                *float64        `json:"wattage"` // custom serialize_power → f64
	TuningPercent          *uint8          `json:"tuning_percent"`
	TuningTarget           *TuningTarget   `json:"tuning_target"`
	ScaledTuningTarget     *TuningTarget   `json:"scaled_tuning_target"`
	TuningCapabilities     json.RawMessage `json:"tuning_capabilities"`
	Efficiency             *float64        `json:"efficiency"`
	LightFlashing          *bool           `json:"light_flashing"`
	Messages               []MinerMessage  `json:"messages"`
	Uptime                 *DurationSecs   `json:"uptime"`
	IsMining               bool            `json:"is_mining"`
	Pools                  []PoolGroupData `json:"pools"`
}

// HashrateTH returns current hashrate in TH/s, or 0 if unknown.
func (d MinerData) HashrateTH() float64 {
	if d.Hashrate == nil {
		return 0
	}
	return d.Hashrate.TH()
}

// TimestampTime returns the data timestamp as time.Time (Unix seconds).
func (d MinerData) TimestampTime() time.Time {
	return time.Unix(int64(d.Timestamp), 0).UTC()
}

// ---------------------------------------------------------------------------
// Config types (JSON in/out for set/get config APIs)
// ---------------------------------------------------------------------------

// PoolConfig is a single pool endpoint for set_pools_config.
type PoolConfig struct {
	URL      any    `json:"url"` // PoolURL object or string
	Username string `json:"username"`
	Password string `json:"password"`
}

// PoolGroupConfig is a named group of pools.
type PoolGroupConfig struct {
	Name  string       `json:"name"`
	Quota uint32       `json:"quota"`
	Pools []PoolConfig `json:"pools"`
}

// ScalingConfig controls power/hashrate step scaling.
type ScalingConfig struct {
	Step             uint32   `json:"step"`
	Minimum          uint32   `json:"minimum"`
	Shutdown         *bool    `json:"shutdown"`
	ShutdownDuration *float32 `json:"shutdown_duration"`
}

// TuningConfig is a target plus optional algorithm string.
type TuningConfig struct {
	Target    TuningTarget `json:"target"`
	Algorithm *string      `json:"algorithm"`
}

// FanConfig is a tagged enum: Auto or Manual.
//
//	{"mode":"Auto","target_temp":65.0,"idle_speed":30}
//	{"mode":"Manual","fan_speed":80}
type FanConfig struct {
	Mode        string  `json:"mode"` // "Auto" or "Manual"
	TargetTemp  *float64 `json:"target_temp,omitempty"`
	IdleSpeed   *uint64  `json:"idle_speed,omitempty"`
	FanSpeed    *uint64  `json:"fan_speed,omitempty"`
}

// NewFanConfigAuto builds an automatic fan config.
func NewFanConfigAuto(targetTemp float64, idleSpeed *uint64) FanConfig {
	return FanConfig{Mode: "Auto", TargetTemp: &targetTemp, IdleSpeed: idleSpeed}
}

// NewFanConfigManual builds a manual fan speed config.
func NewFanConfigManual(fanSpeed uint64) FanConfig {
	return FanConfig{Mode: "Manual", FanSpeed: &fanSpeed}
}
