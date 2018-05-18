// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Period      time.Duration `config:"period"`
	Domains     []string      `config:"domains" validate:"required"`
	TMAdmin     string        `config:"tmadmin"`
	PrintServer bool          `config:"printserver"`
	PrintClient bool          `config:"printclient"`
	PrintQueue  bool          `config:"printqueue"`
}

var DefaultConfig = Config{
	Period:      10 * time.Second,
	PrintServer: true,
	PrintClient: true,
	PrintQueue:  true,
	TMAdmin:     "tmadmin",
}
