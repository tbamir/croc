package models

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/schollz/croc/v10/src/utils"
	log "github.com/schollz/logger"
)

// TCP_BUFFER_SIZE is the maximum packet size
const TCP_BUFFER_SIZE = 1024 * 64

// DEFAULT_RELAY is the default relay used (can be set using --relay)
var (
	DEFAULT_RELAY      = "croc.schollz.com"
	DEFAULT_RELAY6     = "croc6.schollz.com"
	DEFAULT_PORT       = "9009"
	DEFAULT_PASSPHRASE = "pass123"
	INTERNAL_DNS       = false // DISABLED for corporate network compatibility
)

// publicDNS are servers to be queried if a local lookup fails
// DISABLED for corporate network compatibility - these are often blocked
var publicDNS = []string{
	// External DNS disabled for corporate networks
	// "1.0.0.1",                // Cloudflare - BLOCKED
	// "1.1.1.1",                // Cloudflare - BLOCKED
	// "8.8.4.4",                // Google - BLOCKED
	// "8.8.8.8",                // Google - BLOCKED
}

func getConfigFile(requireValidPath bool) (fname string, err error) {
	configFile, err := utils.GetConfigDir(requireValidPath)
	if err != nil {
		return
	}
	fname = path.Join(configFile, "internal-dns")
	return
}

func init() {
	log.SetLevel("info")
	log.SetOutput(os.Stderr)
	doRemember := false

	// CORPORATE NETWORK MODE: Always use local DNS, never external
	INTERNAL_DNS = true // Use local DNS only for corporate networks

	for _, flag := range os.Args {
		if flag == "--internal-dns" {
			INTERNAL_DNS = true
			break
		}
		if flag == "--remember" {
			doRemember = true
		}
	}
	if doRemember {
		// save in config file
		fname, err := getConfigFile(true)
		if err == nil {
			f, _ := os.Create(fname)
			f.Close()
		}
	}
	if !INTERNAL_DNS {
		fname, err := getConfigFile(false)
		if err == nil {
			INTERNAL_DNS = utils.Exists(fname)
		}
	}
	log.Trace("Using internal DNS: ", INTERNAL_DNS)

	// Use HTTPS-compatible ports for corporate network compatibility
	// These provide better firewall traversal than direct IP addresses
	DEFAULT_RELAY = "croc.schollz.com:443"   // Use HTTPS port for firewall compatibility
	DEFAULT_RELAY6 = "croc6.schollz.com:443" // Use HTTPS port for firewall compatibility

	log.Tracef("Default relay (corporate mode): %s", DEFAULT_RELAY)
	log.Tracef("Default relay6 (corporate mode): %s", DEFAULT_RELAY6)
}

// Resolve a hostname to an IP address using DNS.
func lookup(address string) (ipaddress string, err error) {
	// CORPORATE NETWORK MODE: Always use local DNS only
	log.Tracef("Using local DNS only for corporate network compatibility: %s", address)
	return localLookupIP(address)
}

// localLookupIP returns a host's IP address using the local DNS configuration.
func localLookupIP(address string) (ipaddress string, err error) {
	// ENHANCED: Increased timeout for restrictive networks (libraries, hotels, etc.)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) // Increased timeout
	defer cancel()

	r := &net.Resolver{}

	// Use the context with timeout in the LookupHost function
	ip, err := r.LookupHost(ctx, address)
	if err != nil {
		// Fallback for corporate networks - return the address as-is if it looks like an IP
		if net.ParseIP(address) != nil {
			return address, nil
		}
		return "", fmt.Errorf("corporate network DNS resolution failed for %s: %w", address, err)
	}
	ipaddress = ip[0]
	return
}

// remoteLookupIP - DISABLED for corporate network compatibility
func remoteLookupIP(address, dns string) (ipaddress string, err error) {
	// CORPORATE NETWORK MODE: External DNS disabled
	return "", fmt.Errorf("external DNS disabled for corporate network compatibility")
}
