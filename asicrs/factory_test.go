package asicrs

import (
	"testing"
)

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("empty version")
	}
	t.Logf("asic-rs-ffi version: %s", v)
}

func TestFactoryHostsFromSubnet(t *testing.T) {
	f, err := NewFactoryFromSubnet("10.0.0.0/30")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// /30 has 4 addresses (network + broadcast + 2 hosts depending on ipnet)
	if f.IsEmpty() {
		t.Fatal("expected hosts from /30")
	}
	n := f.Len()
	if n <= 0 {
		t.Fatalf("Len() = %d", n)
	}
	hosts, err := f.Hosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != n {
		t.Fatalf("Hosts len %d != Len %d", len(hosts), n)
	}
	t.Logf("hosts (%d): %v", len(hosts), hosts)
}

func TestFactoryFromRangeAndOctets(t *testing.T) {
	f, err := NewFactoryFromRange("192.168.1.1-3")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if f.Len() != 3 {
		t.Fatalf("range Len = %d, want 3", f.Len())
	}

	f2, err := NewFactoryFromOctets("10", "0", "0", "1-2")
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	if f2.Len() != 2 {
		t.Fatalf("octets Len = %d, want 2", f2.Len())
	}
}

func TestFactoryWithSubnetAppend(t *testing.T) {
	f := NewFactory()
	defer f.Close()
	if !f.IsEmpty() {
		t.Fatal("new factory should be empty")
	}
	if err := f.WithSubnet("172.16.0.0/30"); err != nil {
		t.Fatal(err)
	}
	n1 := f.Len()
	if err := f.WithRange("172.16.1.1-2"); err != nil {
		t.Fatal(err)
	}
	if f.Len() <= n1 {
		t.Fatalf("expected more hosts after append, got %d then %d", n1, f.Len())
	}
}

func TestFactoryInvalidSubnet(t *testing.T) {
	_, err := NewFactoryFromSubnet("not-a-subnet")
	if err == nil {
		t.Fatal("expected error for invalid subnet")
	}
}

func TestFactoryTuningOptions(t *testing.T) {
	f := NewFactory()
	defer f.Close()
	f.SetPortCheck(false)
	f.SetConcurrentLimit(50)
	f.SetIdentificationTimeoutSecs(5)
	f.SetConnectivityTimeoutSecs(1)
	f.SetConnectivityRetries(1)
	f.SetNofileAdjustment(false)
	f.SetNofileLimit(4096)
	if err := f.WithSubnet("10.255.255.0/30"); err != nil {
		t.Fatal(err)
	}
	f.SetAdaptiveConcurrency()
}

func TestClosedFactory(t *testing.T) {
	f := NewFactory()
	f.Close()
	if err := f.WithSubnet("10.0.0.0/30"); err == nil {
		t.Fatal("expected error on closed factory")
	}
	if _, err := f.GetMiner("10.0.0.1"); err == nil {
		t.Fatal("expected error on closed factory GetMiner")
	}
}
