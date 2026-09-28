package netutil

import (
	"net"
	"testing"
)

func TestDetectInterfaceType(t *testing.T) {
	tests := []struct {
		name     string
		iface    string
		ipStr    string
		expected InterfaceType
	}{
		{
			name:     "iOS USB Tethering subnet",
			iface:    "enx525400123456",
			ipStr:    "172.20.10.2",
			expected: InterfaceTypeUSBiOS,
		},
		{
			name:     "iOS USB Windows adapter name",
			iface:    "Apple Mobile Device Ethernet",
			ipStr:    "172.20.10.3",
			expected: InterfaceTypeUSBiOS,
		},
		{
			name:     "iOS USB Linux ipheth name",
			iface:    "ipheth0",
			ipStr:    "172.20.10.5",
			expected: InterfaceTypeUSBiOS,
		},
		{
			name:     "Android AOSP USB Tethering subnet",
			iface:    "usb0",
			ipStr:    "192.168.42.15",
			expected: InterfaceTypeUSBAndroid,
		},
		{
			name:     "Android RNDIS adapter name",
			iface:    "rndis0",
			ipStr:    "192.168.43.10",
			expected: InterfaceTypeUSBAndroid,
		},
		{
			name:     "Standard home LAN Wi-Fi",
			iface:    "wlan0",
			ipStr:    "192.168.1.108",
			expected: InterfaceTypeLAN,
		},
		{
			name:     "Corporate 10.x LAN Ethernet",
			iface:    "eth0",
			ipStr:    "10.0.1.50",
			expected: InterfaceTypeLAN,
		},
		{
			name:     "Loopback",
			iface:    "lo",
			ipStr:    "127.0.0.1",
			expected: InterfaceTypeLoopback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ipStr)
			if ip == nil {
				t.Fatalf("failed to parse IP %s", tc.ipStr)
			}
			got := DetectInterfaceType(tc.iface, ip)
			if got != tc.expected {
				t.Errorf("DetectInterfaceType(%q, %q) = %v; want %v", tc.iface, tc.ipStr, got, tc.expected)
			}
		})
	}
}

func TestSelectInterface(t *testing.T) {
	candidates := []InterfaceInfo{
		{Name: "wlan0", IP: "192.168.1.108", Type: InterfaceTypeLAN},
		{Name: "ipheth0", IP: "172.20.10.2", Type: InterfaceTypeUSBiOS},
		{Name: "usb0", IP: "192.168.42.15", Type: InterfaceTypeUSBAndroid},
	}

	// 1. Auto mode prefers iOS USB over Android USB over LAN
	chosenAuto := SelectInterface(candidates, "auto")
	if chosenAuto.Type != InterfaceTypeUSBiOS || chosenAuto.IP != "172.20.10.2" {
		t.Errorf("expected auto to select iOS USB 172.20.10.2, got %+v", chosenAuto)
	}

	// 2. Auto mode with only Android USB and LAN prefers Android USB
	androidAndLAN := []InterfaceInfo{
		{Name: "wlan0", IP: "192.168.1.108", Type: InterfaceTypeLAN},
		{Name: "usb0", IP: "192.168.42.15", Type: InterfaceTypeUSBAndroid},
	}
	chosenAndroid := SelectInterface(androidAndLAN, "auto")
	if chosenAndroid.Type != InterfaceTypeUSBAndroid || chosenAndroid.IP != "192.168.42.15" {
		t.Errorf("expected auto to select Android USB, got %+v", chosenAndroid)
	}

	// 3. Auto mode with only LAN selects LAN
	lanOnly := []InterfaceInfo{
		{Name: "wlan0", IP: "192.168.1.108", Type: InterfaceTypeLAN},
	}
	chosenLAN := SelectInterface(lanOnly, "auto")
	if chosenLAN.Type != InterfaceTypeLAN || chosenLAN.IP != "192.168.1.108" {
		t.Errorf("expected auto to select LAN, got %+v", chosenLAN)
	}

	// 4. Force LAN mode ignores USB interfaces
	forcedLAN := SelectInterface(candidates, "lan")
	if forcedLAN.Type != InterfaceTypeLAN || forcedLAN.IP != "192.168.1.108" {
		t.Errorf("expected forced lan to select 192.168.1.108, got %+v", forcedLAN)
	}

	// 5. Force USB mode selects USB
	forcedUSB := SelectInterface(candidates, "usb")
	if forcedUSB.Type != InterfaceTypeUSBiOS {
		t.Errorf("expected forced usb to select iOS USB, got %+v", forcedUSB)
	}

	// 6. Empty candidates falls back to loopback
	fallback := SelectInterface(nil, "auto")
	if fallback.IP != "127.0.0.1" || fallback.Type != InterfaceTypeLoopback {
		t.Errorf("expected fallback to 127.0.0.1 loopback, got %+v", fallback)
	}
}

func TestResolveServerIPOverride(t *testing.T) {
	info, err := ResolveServerIP("auto", "192.168.1.200")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IP != "192.168.1.200" || info.Type != InterfaceTypeCustom {
		t.Errorf("expected manual IP override, got %+v", info)
	}
}
