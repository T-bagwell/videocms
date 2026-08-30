package media

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ssdpAddr = "239.255.255.250:1900"
	// DLNADeviceType is the UPnP device type advertised over SSDP.
	DLNADeviceType = "urn:schemas-upnp-org:device:MediaServer:1"
)

// DLNAManager announces the media server over SSDP so UPnP/DLNA players on the
// LAN can discover it and browse the library through /dlna/* HTTP endpoints.
type DLNAManager struct {
	friendlyName string
	port         string
	allowedIPs   map[string]struct{}
	server       string
	uuid         string
}

func NewDLNAManager(friendlyName, port string, allowedIPs []string) *DLNAManager {
	set := make(map[string]struct{}, len(allowedIPs))
	for _, ip := range allowedIPs {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			set[ip] = struct{}{}
		}
	}
	if friendlyName == "" {
		friendlyName = "VideoCMS"
	}
	return &DLNAManager{
		friendlyName: friendlyName,
		port:         port,
		allowedIPs:   set,
		server:       "VideoCMS/" + "1.0 UPnP/1.0",
		uuid:         uuid.NewSHA1(uuid.NameSpaceURL, []byte("videocms-dlna")).String(),
	}
}

// FriendlyName returns the name shown to DLNA clients.
func (m *DLNAManager) FriendlyName() string {
	return m.friendlyName
}

// UDN returns the stable unique device name for this installation.
func (m *DLNAManager) UDN() string {
	return m.uuid
}

// Allowed reports whether a remote address may use DLNA endpoints.
func (m *DLNAManager) Allowed(remoteAddr string) bool {
	if len(m.allowedIPs) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if _, ok := m.allowedIPs[host]; ok {
		return true
	}
	// Accept a CIDR-style prefix (e.g. "192.168.1.0/24").
	for allowed := range m.allowedIPs {
		if _, ipnet, err := net.ParseCIDR(allowed); err == nil {
			if ip := net.ParseIP(host); ip != nil && ipnet.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// Start runs the SSDP responder until ctx is cancelled. Listen failures are
// logged but never fatal: DLNA simply stays unavailable.
func (m *DLNAManager) Start(ctx context.Context) {
	go m.respondToMSearch(ctx)
	go m.announceAlive(ctx)
}

func (m *DLNAManager) respondToMSearch(ctx context.Context) {
	conn, err := net.ListenPacket("udp4", "0.0.0.0:1900")
	if err != nil {
		log.Printf("dlna: ssdp listen: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			continue
		}
		msg := string(buf[:n])
		if !strings.Contains(msg, "M-SEARCH") {
			continue
		}
		st := parseSSDPST(msg)
		if st != "ssdp:all" && st != DLNADeviceType {
			continue
		}
		if !m.Allowed(addr.String()) {
			continue
		}
		location := m.location()
		body := fmt.Sprintf(
			"HTTP/1.1 200 OK\r\n"+
				"CACHE-CONTROL: max-age=1800\r\n"+
				"EXT:\r\n"+
				"LOCATION: %s\r\n"+
				"SERVER: %s\r\n"+
				"ST: %s\r\n"+
				"USN: uuid:%s::%s\r\n\r\n",
			location, m.server, DLNADeviceType, m.uuid, DLNADeviceType)
		if _, err := conn.WriteTo([]byte(body), addr); err != nil {
			log.Printf("dlna: ssdp reply: %v", err)
		}
	}
}

func (m *DLNAManager) announceAlive(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		log.Printf("dlna: resolve multicast: %v", err)
		return
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		log.Printf("dlna: multicast dial: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()
	base := "NOTIFY * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"CACHE-CONTROL: max-age=1800\r\n" +
		"LOCATION: %s\r\n" +
		"SERVER: %s\r\n" +
		"NT: %s\r\n" +
		"NTS: ssdp:alive\r\n" +
		"USN: uuid:%s::%s\r\n\r\n"
	device := fmt.Sprintf(base, m.location(), m.server,
		DLNADeviceType, m.uuid, DLNADeviceType)
	root := fmt.Sprintf(base, m.location(), m.server,
		"upnp:rootdevice", m.uuid, "upnp:rootdevice")
	ticker := time.NewTicker(90 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if _, err := conn.Write([]byte(device)); err != nil {
			log.Printf("dlna: announce: %v", err)
		}
		if _, err := conn.Write([]byte(root)); err != nil {
			log.Printf("dlna: announce: %v", err)
		}
	}
}

// location builds the externally reachable base URL for the device description.
func (m *DLNAManager) location() string {
	host := lanIP()
	return "http://" + host + ":" + m.port + "/dlna/device.xml"
}

func (m *DLNAManager) BaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func parseSSDPST(msg string) string {
	for _, line := range strings.Split(msg, "\r\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "ST") {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// lanIP returns the first non-loopback IPv4 address, falling back to
// loopback so announcements always carry a usable LOCATION.
func lanIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "127.0.0.1"
}
