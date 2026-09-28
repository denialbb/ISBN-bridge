package netutil

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// InterfaceType classifies a network interface connection type.
type InterfaceType string

const (
	InterfaceTypeUSBiOS     InterfaceType = "usb_ios"
	InterfaceTypeUSBAndroid InterfaceType = "usb_android"
	InterfaceTypeLAN        InterfaceType = "lan"
	InterfaceTypeLoopback   InterfaceType = "loopback"
	InterfaceTypeCustom     InterfaceType = "custom"
)

// InterfaceInfo contains network address details and its interface classification.
type InterfaceInfo struct {
	Name string        `json:"name"`
	IP   string        `json:"ip"`
	Type InterfaceType `json:"type"`
}

var (
	_, iosSubnet, _        = net.ParseCIDR("172.20.10.0/28")
	_, androidSubnet42, _  = net.ParseCIDR("192.168.42.0/24")
	_, androidSubnet43, _  = net.ParseCIDR("192.168.43.0/24")
	_, androidSubnet44, _  = net.ParseCIDR("192.168.44.0/24")
	_, androidSubnet225, _ = net.ParseCIDR("192.168.225.0/24")
)

// DetectInterfaceType evaluates an interface name and IPv4 address to determine its connection class.
func DetectInterfaceType(ifaceName string, ip net.IP) InterfaceType {
	if ip == nil {
		return InterfaceTypeLoopback
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return InterfaceTypeLoopback
	}

	if ip4.IsLoopback() {
		return InterfaceTypeLoopback
	}

	lowerName := strings.ToLower(ifaceName)

	// 1. iOS USB Personal Hotspot: standard 172.20.10.0/28 or Apple / ipheth interface name
	if iosSubnet.Contains(ip4) || strings.Contains(lowerName, "apple") || strings.Contains(lowerName, "ipheth") {
		return InterfaceTypeUSBiOS
	}

	// 2. Android USB Tethering: 192.168.42-44.x, 225.x or RNDIS / USB interface name
	if androidSubnet42.Contains(ip4) || androidSubnet43.Contains(ip4) || androidSubnet44.Contains(ip4) || androidSubnet225.Contains(ip4) ||
		strings.Contains(lowerName, "rndis") || (strings.Contains(lowerName, "usb") && !strings.Contains(lowerName, "wlan")) {
		return InterfaceTypeUSBAndroid
	}

	return InterfaceTypeLAN
}

// GetCandidateInterfaces queries all active, non-loopback network interfaces and returns
// their IPv4 candidates classified by type.
func GetCandidateInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var candidates []InterfaceInfo
	for _, iface := range ifaces {
		// Interface must be up and not purely loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
				continue
			}

			candidates = append(candidates, InterfaceInfo{
				Name: iface.Name,
				IP:   ip4.String(),
				Type: DetectInterfaceType(iface.Name, ip4),
			})
		}
	}

	return candidates, nil
}

// SelectInterface chooses the best interface according to policy and mode.
// Supported modes:
//   - "auto" (default): Prefer USB_iOS, then USB_Android, then LAN, fallback to 127.0.0.1.
//   - "usb": Exclusively choose USB_iOS or USB_Android.
//   - "lan": Exclusively choose LAN (ignore USB tethering).
func SelectInterface(candidates []InterfaceInfo, mode string) InterfaceInfo {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}

	var (
		iosUSB     *InterfaceInfo
		androidUSB *InterfaceInfo
		lan        *InterfaceInfo
	)

	for i := range candidates {
		c := &candidates[i]
		switch c.Type {
		case InterfaceTypeUSBiOS:
			if iosUSB == nil {
				iosUSB = c
			}
		case InterfaceTypeUSBAndroid:
			if androidUSB == nil {
				androidUSB = c
			}
		case InterfaceTypeLAN:
			if lan == nil {
				lan = c
			}
		}
	}

	switch mode {
	case "usb":
		if iosUSB != nil {
			return *iosUSB
		}
		if androidUSB != nil {
			return *androidUSB
		}
	case "lan":
		if lan != nil {
			return *lan
		}
	default: // "auto"
		if iosUSB != nil {
			return *iosUSB
		}
		if androidUSB != nil {
			return *androidUSB
		}
		if lan != nil {
			return *lan
		}
	}

	// Fallback to loopback
	return InterfaceInfo{
		Name: "lo",
		IP:   "127.0.0.1",
		Type: InterfaceTypeLoopback,
	}
}

// ResolveServerIP determines the target IP for the server base URL using the active configuration.
// If overrideIP is non-empty, it takes precedence.
func ResolveServerIP(mode string, overrideIP string) (InterfaceInfo, error) {
	if trimmed := strings.TrimSpace(overrideIP); trimmed != "" {
		return InterfaceInfo{
			Name: "manual",
			IP:   trimmed,
			Type: InterfaceTypeCustom,
		}, nil
	}

	candidates, err := GetCandidateInterfaces()
	if err != nil {
		return InterfaceInfo{Name: "lo", IP: "127.0.0.1", Type: InterfaceTypeLoopback}, err
	}

	// If mode is "auto" or "lan" and no USB is selected (or lan is preferred),
	// check if outbound UDP dialing gives us a specific primary LAN interface.
	selected := SelectInterface(candidates, mode)
	if (mode == "lan" || mode == "auto") && selected.Type == InterfaceTypeLAN {
		if outbound := GetOutboundLANIP(); outbound != "" {
			for _, c := range candidates {
				if c.IP == outbound {
					return c, nil
				}
			}
			return InterfaceInfo{
				Name: "outbound",
				IP:   outbound,
				Type: InterfaceTypeLAN,
			}, nil
		}
	}

	return selected, nil
}

// GetOutboundLANIP attempts to determine the primary outbound IP via standard UDP routing.
func GetOutboundLANIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 400*time.Millisecond)
	if err == nil {
		defer conn.Close()
		if udpAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return udpAddr.IP.String()
		}
	}
	return ""
}

// FormatServerURL formats an HTTP base URL from IP and port.
func FormatServerURL(ip string, port int) string {
	if strings.Contains(ip, ":") {
		return fmt.Sprintf("http://[%s]:%d", ip, port)
	}
	return fmt.Sprintf("http://%s:%d", ip, port)
}
