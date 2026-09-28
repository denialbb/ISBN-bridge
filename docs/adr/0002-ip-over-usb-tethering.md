# ADR 0002: IP-Over-USB Tethering Architecture for Cross-Platform Mobile Comms

## Status
Accepted

## Context
ISBN Bridge communicates scans via HTTP from mobile clients (iOS Shortcuts, Android HTTP Shortcuts) to a local Go server listening on port 8765. In corporate networks, guest Wi-Fi networks with client isolation, or environments without a common local Wi-Fi router, communication between phone and PC over LAN is blocked or unavailable.

Physical USB cable connection is universally available across Windows and Linux. However:
1. iOS does not allow unprivileged apps or Apple Shortcuts to communicate over raw USB sockets, serial/COM ports, or WebUSB/WebHID.
2. Apple Shortcuts natively supports HTTP requests (`Get Contents of URL`).
3. When an iPhone is connected via USB with "Personal Hotspot" enabled ("USB Only"), iOS creates a direct point-to-point network adapter (NDIS/ipheth) on the host with standard subnet `172.20.10.0/28`.
4. Similarly, Android USB tethering exposes a standard RNDIS/CDC-NCM adapter (typically `192.168.42.0/24` or vendor-specific variants).

## Decision
1. **Transport Layer**: Adopt IP-over-USB via standard OS USB tethering (Apple Mobile Device Ethernet / `ipheth` on iOS; RNDIS/NCM on Android). This requires zero native mobile apps, preserves existing Apple Shortcuts and Android HTTP Shortcuts, and operates completely offline without needing cellular data or internet routing.
2. **Interface Resolver (`pkg/netutil`)**:
   - Classify network interfaces into `USB_iOS` (`172.20.10.0/28` or Apple adapter name), `USB_Android` (`192.168.42-44.0/24` or RNDIS/USB adapter name), and `LAN` (standard routable unicast IP).
   - Priority policy (`network_mode = auto`):
     1. Active USB Tethering (iOS first, then Android)
     2. Active Outbound LAN/Wi-Fi (`getOutboundIP`)
     3. Loopback (`127.0.0.1`)
   - Manual override support via `scanner.conf`:
     - `network_mode = auto | usb | lan`
     - `server_ip = <explicit_ip>`
3. **Dynamic Hot-Plugging**:
   - Re-evaluate active interfaces dynamically during QR and pairing requests.
   - Run a lightweight periodic monitor in the Go server. When a USB cable connection or disconnection causes the preferred IP to transition, automatically update the base URL and refresh the desktop QR code popup so the user can scan the updated link immediately without restarting the server.
4. **Dual-Homed Routing Safety**:
   - Host PC remains on its primary LAN gateway for external internet traffic. Only direct scanning requests route across the point-to-point USB subnet.

## Consequences
- Zero code modifications or developer certificates required on iOS; Apple Shortcuts work out-of-the-box over USB cable.
- Works identically across Windows and Linux.
- Seamless automatic fallback to Wi-Fi when USB is unplugged, and automatic elevation to USB when cable is connected.
