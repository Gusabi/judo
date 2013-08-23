// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package instance

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var ErrNoDNSName = errors.New("DNS name not allocated")

// An instance Id is a provider-specific identifier associated with an
// instance (physical or virtual machine allocated in the provider).
type Id string

// Port identifies a network port number for a particular protocol.
type Port struct {
	Protocol string
	Number   int
}

func (p Port) String() string {
	return fmt.Sprintf("%d/%s", p.Number, p.Protocol)
}

// Instance represents the the realization of a machine in state.
type Instance interface {
	// Id returns a provider-generated identifier for the Instance.
	Id() Id

	// Addresses returns a list of hostnames or ip addresses
	// associated with the instance. This will supercede DNSName
	// which can be implemented by selecting a preferred address.
	Addresses() ([]Address, error)

	// DNSName returns the DNS name for the instance.
	// If the name is not yet allocated, it will return
	// an ErrNoDNSName error.
	DNSName() (string, error)

	// WaitDNSName returns the DNS name for the instance,
	// waiting until it is allocated if necessary.
	// TODO: We may not need this in the interface any more.  All
	// implementations now delegate to environs.WaitDNSName.
	WaitDNSName() (string, error)

	// OpenPorts opens the given ports on the instance, which
	// should have been started with the given machine id.
	OpenPorts(machineId string, ports []Port) error

	// ClosePorts closes the given ports on the instance, which
	// should have been started with the given machine id.
	ClosePorts(machineId string, ports []Port) error

	// Ports returns the set of ports open on the instance, which
	// should have been started with the given machine id.
	// The ports are returned as sorted by state.SortPorts.
	Ports(machineId string) ([]Port, error)
}

// HardwareCharacteristics represents the characteristics of the instance (if known).
// Attributes that are nil are unknown or not supported.
type HardwareCharacteristics struct {
	Arch     *string `yaml:"arch,omitempty"`
	Mem      *uint64 `yaml:"mem,omitempty"`
	CpuCores *uint64 `yaml:"cpucores,omitempty"`
	CpuPower *uint64 `yaml:"cpupower,omitempty"`
}

func uintStr(i uint64) string {
	if i == 0 {
		return ""
	}
	return fmt.Sprintf("%d", i)
}

func (hc HardwareCharacteristics) String() string {
	var strs []string
	if hc.Arch != nil {
		strs = append(strs, "arch="+*hc.Arch)
	}
	if hc.CpuCores != nil {
		strs = append(strs, "cpu-cores="+uintStr(*hc.CpuCores))
	}
	if hc.CpuPower != nil {
		strs = append(strs, "cpu-power="+uintStr(*hc.CpuPower))
	}
	if hc.Mem != nil {
		s := uintStr(*hc.Mem)
		if s != "" {
			s += "M"
		}
		strs = append(strs, "mem="+s)
	}
	return strings.Join(strs, " ")
}

// MustParseHardware constructs a HardwareCharacteristics from the supplied arguments,
// as Parse, but panics on failure.
func MustParseHardware(args ...string) HardwareCharacteristics {
	hc, err := ParseHardware(args...)
	if err != nil {
		panic(err)
	}
	return hc
}

// ParseHardware constructs a HardwareCharacteristics from the supplied arguments,
// each of which must contain only spaces and name=value pairs. If any
// name is specified more than once, an error is returned.
func ParseHardware(args ...string) (HardwareCharacteristics, error) {
	hc := HardwareCharacteristics{}
	for _, arg := range args {
		raws := strings.Split(strings.TrimSpace(arg), " ")
		for _, raw := range raws {
			if raw == "" {
				continue
			}
			if err := hc.setRaw(raw); err != nil {
				return HardwareCharacteristics{}, err
			}
		}
	}
	return hc, nil
}

// setRaw interprets a name=value string and sets the supplied value.
func (hc *HardwareCharacteristics) setRaw(raw string) error {
	eq := strings.Index(raw, "=")
	if eq <= 0 {
		return fmt.Errorf("malformed characteristic %q", raw)
	}
	name, str := raw[:eq], raw[eq+1:]
	var err error
	switch name {
	case "arch":
		err = hc.setArch(str)
	case "cpu-cores":
		err = hc.setCpuCores(str)
	case "cpu-power":
		err = hc.setCpuPower(str)
	case "mem":
		err = hc.setMem(str)
	default:
		return fmt.Errorf("unknown characteristic %q", name)
	}
	if err != nil {
		return fmt.Errorf("bad %q characteristic: %v", name, err)
	}
	return nil
}

func (hc *HardwareCharacteristics) setArch(str string) error {
	if hc.Arch != nil {
		return fmt.Errorf("already set")
	}
	switch str {
	case "":
	case "amd64", "i386", "arm":
	default:
		return fmt.Errorf("%q not recognized", str)
	}
	hc.Arch = &str
	return nil
}

func (hc *HardwareCharacteristics) setCpuCores(str string) (err error) {
	if hc.CpuCores != nil {
		return fmt.Errorf("already set")
	}
	hc.CpuCores, err = parseUint64(str)
	return
}

func (hc *HardwareCharacteristics) setCpuPower(str string) (err error) {
	if hc.CpuPower != nil {
		return fmt.Errorf("already set")
	}
	hc.CpuPower, err = parseUint64(str)
	return
}

func (hc *HardwareCharacteristics) setMem(str string) error {
	if hc.Mem != nil {
		return fmt.Errorf("already set")
	}
	var value uint64
	if str != "" {
		mult := 1.0
		if m, ok := mbSuffixes[str[len(str)-1:]]; ok {
			str = str[:len(str)-1]
			mult = m
		}
		val, err := strconv.ParseFloat(str, 64)
		if err != nil || val < 0 {
			return fmt.Errorf("must be a non-negative float with optional M/G/T/P suffix")
		}
		val *= mult
		value = uint64(math.Ceil(val))
	}
	hc.Mem = &value
	return nil
}

func parseUint64(str string) (*uint64, error) {
	var value uint64
	if str != "" {
		if val, err := strconv.ParseUint(str, 10, 64); err != nil {
			return nil, fmt.Errorf("must be a non-negative integer")
		} else {
			value = uint64(val)
		}
	}
	return &value, nil
}

var mbSuffixes = map[string]float64{
	"M": 1,
	"G": 1024,
	"T": 1024 * 1024,
	"P": 1024 * 1024 * 1024,
}
