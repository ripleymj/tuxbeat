// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Period      time.Duration `config:"period"`
	Domains     []string      `config:"domains" validate:"required"`
	printserver bool          `config:"printserver"`
	printclient bool          `config:"printclient"`
	printqueue  bool          `config:"printqueue"`
}

var DefaultConfig = Config{
	Period:      10 * time.Second,
	printserver: true,
	printclient: true,
	printqueue:  true,
}
