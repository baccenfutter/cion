package resolve

import (
	"log"
	"os"
	"path/filepath"

	"github.com/miekg/dns"
)

type (
	// Resolver is a small in-memory DNS resolver.
	Resolver struct {
		RootDomain string
		ZonePath   string
	}
)

func (r Resolver) zoneFQDN(name string) string {
	return name + "." + r.RootDomain + "."
}

// ZoneExists returns true if a zone with the given name exists
func (r Resolver) ZoneExists(name string) bool {
	filePath := filepath.Join(
		r.ZonePath,
		r.zoneFQDN(name)+"zone")
	if _, err := os.Stat(filePath); err == nil {
		return true
	}
	return false
}

// RecordExists returns true if a record
func (r Resolver) RecordExists(record, zone string) bool {
	if !r.ZoneExists(zone) {
		return false
	}
	m := new(dns.Msg)
	m.SetQuestion(record+"."+r.zoneFQDN(zone), dns.TypeSRV)
	c := new(dns.Client)
	in, rtt, err := c.Exchange(m, "127.0.0.1:53")
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println(in, rtt)
	return false
}
