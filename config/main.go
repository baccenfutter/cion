package config

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

// Specification contains the configuration variables.
type Specification struct {
	KeyDir     string `envconfig:"key_dir"`
	ConfDir    string `envconfig:"conf_dir"`
	ZoneDir    string `envconfig:"zone_dir"`
	RootDomain string `required:"true" envconfig:"root_domain"`
	TTL        uint
}

// Config reads and returns the configuration from the environment
func Config() *Specification {
	s := &Specification{
		KeyDir:  "/etc/bind/keys",
		ConfDir: "/etc/bind/zones",
		ZoneDir: "/var/bind/dyn",
		TTL:     180,
	}
	err := envconfig.Process("cion", s)
	if err != nil {
		log.Fatal(err)
	}
	return s
}
