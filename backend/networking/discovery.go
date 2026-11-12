package networking

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"familycontrol/backend/protection"
	"familycontrol/backend/windows"
)

const DiscoveryProtocolVersion = "FamilyControl-LAN-v1"

// DiscoveryBeacon contains strictly non-sensitive public announcement metadata for LAN discovery.
type DiscoveryBeacon struct {
	Protocol        string `json:"protocol"`
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	Hostname        string `json:"hostname"`
	OSVersion       string `json:"osVersion"`
	Mode            string `json:"mode"` // "parent" | "child"
	IPAddress       string `json:"ipAddress"`
	TLSPort         int    `json:"tlsPort"`
	CertFingerprint string `json:"certFingerprint"`
	Paired          bool   `json:"paired"`
	LastSeen        string `json:"lastSeen"`
}

// DiscoveryService handles UDP broadcast announcements and peer discovery on the local home network.
type DiscoveryService struct {
	mu       sync.RWMutex
	port     int
	peers    map[string]DiscoveryBeacon
	stopCh   chan struct{}
	running  bool
	beaconFn func() DiscoveryBeacon
}

func NewDiscoveryService(port int, beaconProvider func() DiscoveryBeacon) *DiscoveryService {
	if port <= 0 {
		port = 41880
	}
	return &DiscoveryService{
		port:     port,
		peers:    make(map[string]DiscoveryBeacon),
		stopCh:   make(chan struct{}),
		beaconFn: beaconProvider,
	}
}

func (ds *DiscoveryService) Start() error {
	ds.mu.Lock()
	if ds.running {
		ds.mu.Unlock()
		return nil
	}
	ds.running = true
	ds.stopCh = make(chan struct{})
	ds.mu.Unlock()

	addr := &net.UDPAddr{IP: net.IPv4zero, Port: ds.port}
	conn, err := net.ListenUDP("udp4", addr)
	if err == nil {
		go ds.listenLoop(conn)
	}
	go ds.broadcastLoop()
	return nil
}

func (ds *DiscoveryService) Stop() {
	ds.mu.Lock()
	if !ds.running {
		ds.mu.Unlock()
		return
	}
	ds.running = false
	close(ds.stopCh)
	ds.mu.Unlock()
}

// BroadcastOnce sends a single UDP broadcast beacon across local IPv4 broadcast addresses.
func (ds *DiscoveryService) BroadcastOnce() error {
	if ds.beaconFn == nil {
		return nil
	}
	beacon := ds.beaconFn()
	beacon.Protocol = DiscoveryProtocolVersion
	if beacon.Hostname == "" {
		beacon.Hostname, _ = os.Hostname()
	}
	if beacon.OSVersion == "" {
		beacon.OSVersion = windows.GetOSVersionString()
	}
	data, err := json.Marshal(beacon)
	if err != nil {
		return err
	}

	targets := getBroadcastTargets(ds.port)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}
	defer conn.Close()

	for _, t := range targets {
		_, _ = conn.WriteToUDP(data, t)
	}
	return nil
}

// ListDiscoveredPeers returns all recently discovered FamilyControl nodes on the LAN.
func (ds *DiscoveryService) ListDiscoveredPeers(filterMode string) []DiscoveryBeacon {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	var list []DiscoveryBeacon
	for id, peer := range ds.peers {
		ts, err := time.Parse(time.RFC3339, peer.LastSeen)
		if err == nil && ts.Before(cutoff) {
			delete(ds.peers, id)
			continue
		}
		if filterMode != "" && peer.Mode != filterMode {
			continue
		}
		list = append(list, peer)
	}
	if list == nil {
		list = []DiscoveryBeacon{}
	}
	return list
}

// RecordPeer allows manual or unit-test registration of a discovered peer beacon.
func (ds *DiscoveryService) RecordPeer(b DiscoveryBeacon, remoteIP string) {
	if b.Protocol != DiscoveryProtocolVersion || b.DeviceID == "" {
		return
	}
	if ds.beaconFn != nil {
		self := ds.beaconFn()
		if b.DeviceID == self.DeviceID {
			return
		}
	}
	if remoteIP != "" {
		b.IPAddress = remoteIP
	}
	b.LastSeen = time.Now().UTC().Format(time.RFC3339)

	ds.mu.Lock()
	ds.peers[b.DeviceID] = b
	ds.mu.Unlock()
}

func (ds *DiscoveryService) listenLoop(conn *net.UDPConn) {
	defer conn.Close()
	buf := make([]byte, 2048)
	for {
		select {
		case <-ds.stopCh:
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		var b DiscoveryBeacon
		if err := json.Unmarshal(buf[:n], &b); err != nil {
			continue
		}
		remoteIP := ""
		if raddr != nil && raddr.IP != nil {
			remoteIP = raddr.IP.String()
		}
		ds.RecordPeer(b, remoteIP)
	}
}

func (ds *DiscoveryService) broadcastLoop() {
	_ = ds.BroadcastOnce()
	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ds.stopCh:
			return
		case <-ticker.C:
			_ = ds.BroadcastOnce()
		}
	}
}

func getBroadcastTargets(port int) []*net.UDPAddr {
	targets := []*net.UDPAddr{
		{IP: net.IPv4bcast, Port: port},
	}
	for _, ipStr := range protection.GetLocalLANIPs() {
		ip := net.ParseIP(ipStr).To4()
		if ip != nil {
			bcast := net.IPv4(ip[0], ip[1], ip[2], 255)
			targets = append(targets, &net.UDPAddr{IP: bcast, Port: port})
		}
	}
	return targets
}

// FormatEndpoint formats an IP and port into a clean host:port string.
func FormatEndpoint(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}
