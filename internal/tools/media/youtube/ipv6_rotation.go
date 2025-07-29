package youtube

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// IPv6Rotator manages IPv6 address rotation for bypassing rate limiting
type IPv6Rotator struct {
	logger            *logrus.Logger
	availablePrefixes []net.IPNet
	availableIPs      []net.IP
}

// NewIPv6Rotator creates a new IPv6 rotator
func NewIPv6Rotator(logger *logrus.Logger) *IPv6Rotator {
	rotator := &IPv6Rotator{
		logger: logger,
	}

	// Detect available IPv6 prefixes
	rotator.detectIPv6Prefixes()

	return rotator
}

// detectIPv6Prefixes detects available IPv6 prefixes on the system
func (r *IPv6Rotator) detectIPv6Prefixes() {
	interfaces, err := net.Interfaces()
	if err != nil {
		r.logger.Debugf("Failed to get network interfaces: %v", err)
		return
	}

	seenPrefixes := make(map[string]bool)

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			r.logger.Debugf("Failed to get addresses for interface %s: %v", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip := ipnet.IP
				// Only consider global unicast IPv6 addresses that are routable
				// Exclude ULA (fc00::/7), link-local (fe80::/10), and deprecated addresses
				if r.isValidGlobalIPv6(ip) {
					// Store the configured IP
					r.availableIPs = append(r.availableIPs, ip)

					// Use /64 prefix for rotation (standard for most ISPs)
					mask := net.CIDRMask(64, 128)
					prefix := net.IPNet{
						IP:   ip.Mask(mask),
						Mask: mask,
					}

					// Avoid duplicate prefixes
					prefixStr := prefix.String()
					if !seenPrefixes[prefixStr] {
						r.availablePrefixes = append(r.availablePrefixes, prefix)
						seenPrefixes[prefixStr] = true
					}

					r.logger.Debugf("Detected IPv6 address: %s on interface %s, prefix: %s",
						ip.String(), iface.Name, prefixStr)
				}
			}
		}
	}

	if len(r.availableIPs) == 0 {
		r.logger.Debug("No global IPv6 addresses detected")
	} else {
		r.logger.Debugf("Detected %d IPv6 addresses across %d prefixes for rotation",
			len(r.availableIPs), len(r.availablePrefixes))
	}
}

// CreateHTTPClientWithRotation creates an HTTP client with IPv6 rotation
func (r *IPv6Rotator) CreateHTTPClientWithRotation() *http.Client {
	if len(r.availableIPs) == 0 {
		// No IPv6 available, return standard client
		r.logger.Debug("No IPv6 addresses available, using standard client")
		return r.createStandardClient()
	}

	// Select a random configured IPv6 address for this request
	selectedIP, err := r.selectRandomConfiguredIP()
	if err != nil {
		r.logger.Debugf("Failed to select IPv6 address: %v, using standard client", err)
		return r.createStandardClient()
	}

	r.logger.Debugf("Attempting IPv6 connection with address: %s", selectedIP.String())

	// Create transport with specific local address and aggressive fallback
	dialer := &net.Dialer{
		Timeout:       5 * time.Second, // Shorter timeout for IPv6
		KeepAlive:     30 * time.Second,
		FallbackDelay: 50 * time.Millisecond, // Quick fallback
		LocalAddr: &net.TCPAddr{
			IP: selectedIP,
		},
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second, // Shorter timeout
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		ForceAttemptHTTP2:     true, // Enable HTTP/2 like modern browsers
		MaxIdleConnsPerHost:   2,
		DisableCompression:    false, // Enable compression like browsers
	}

	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second, // Shorter overall timeout
	}
}

// createStandardClient creates a standard HTTP client without IPv6 binding
func (r *IPv6Rotator) createStandardClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
		},
	}
}

// selectRandomConfiguredIP selects a random configured IPv6 address
func (r *IPv6Rotator) selectRandomConfiguredIP() (net.IP, error) {
	if len(r.availableIPs) == 0 {
		return nil, fmt.Errorf("no IPv6 addresses available")
	}

	// Select a random address from configured addresses
	ipIndex := 0
	if len(r.availableIPs) > 1 {
		buf := make([]byte, 1)
		if _, err := rand.Read(buf); err == nil {
			ipIndex = int(buf[0]) % len(r.availableIPs)
		}
	}

	return r.availableIPs[ipIndex], nil
}

// IsIPv6Available returns true if IPv6 rotation is available
func (r *IPv6Rotator) IsIPv6Available() bool {
	return len(r.availableIPs) > 0
}

// GetAvailablePrefixes returns the detected IPv6 prefixes
func (r *IPv6Rotator) GetAvailablePrefixes() []string {
	var prefixes []string
	for _, prefix := range r.availablePrefixes {
		prefixes = append(prefixes, prefix.String())
	}
	return prefixes
}

// isValidGlobalIPv6 checks if an IPv6 address is suitable for external connections
func (r *IPv6Rotator) isValidGlobalIPv6(ip net.IP) bool {
	if len(ip) != 16 || ip.To4() != nil {
		return false
	}

	// Must be global unicast (not multicast, loopback, etc.)
	if !ip.IsGlobalUnicast() {
		return false
	}

	// Exclude ULA addresses (fc00::/7)
	if isULA(ip) {
		return false
	}

	// Exclude link-local addresses (fe80::/10)
	if isLinkLocal(ip) {
		return false
	}

	// Exclude documentation prefix (2001:db8::/32)
	if isDocumentation(ip) {
		return false
	}

	// Must not be private (additional check for edge cases)
	if ip.IsPrivate() {
		return false
	}

	return true
}

// isULA checks if an IPv6 address is a Unique Local Address (fc00::/7)
func isULA(ip net.IP) bool {
	if len(ip) != 16 {
		return false
	}
	// ULA addresses have the first byte in range fc00::/7 (0xfc or 0xfd)
	return ip[0] == 0xfc || ip[0] == 0xfd
}

// isLinkLocal checks if an IPv6 address is link-local (fe80::/10)
func isLinkLocal(ip net.IP) bool {
	if len(ip) != 16 {
		return false
	}
	// Link-local addresses start with fe80::/10
	return ip[0] == 0xfe && (ip[1]&0xc0) == 0x80
}

// isDocumentation checks if an IPv6 address is in documentation prefix (2001:db8::/32)
func isDocumentation(ip net.IP) bool {
	if len(ip) != 16 {
		return false
	}
	// Documentation prefix 2001:db8::/32
	return ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8
}
