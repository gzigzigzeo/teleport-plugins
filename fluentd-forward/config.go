/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config stores configuration variables
type Config struct {
	// FluentdURL is fluentd url
	FluentdURL string `mapstructure:"fluentd-url"`

	// FluentdCert is a path to fluentd cert
	FluentdCert string `mapstructure:"fluentd-cert"`

	// FluentdKey is a path to fluentd key
	FluentdKey string `mapstructure:"fluentd-key"`

	// FluentdCA is a path to fluentd CA
	FluentdCA string `mapstructure:"fluentd-ca"`

	// TeleportAddr is a Teleport addr
	TeleportAddr string `mapstructure:"teleport-addr"`

	// TeleportIdentityFile is a path to Teleport identity file
	TeleportIdentityFile string `mapstructure:"teleport-identity"`

	// TeleportProfileName is a Teleport profile name
	TeleportProfileName string `mapstructure:"teleport-profile-name"`

	// TeleportProfileDir is a Teleport profile dir
	TeleportProfileDir string `mapstructure:"teleport-profile-dir"`

	// TeleportCA is a path to Teleport CA file
	TeleportCA string `mapstructure:"teleport-ca"`

	// TeleportCert is a path to Teleport cert file
	TeleportCert string `mapstructure:"teleport-cert"`

	// TeleportKey is a path to Teleport key file
	TeleportKey string `mapstructure:"teleport-key"`

	// StorageDir is a path to badger storage dir
	StorageDir string `mapstructure:"storage-dir"`

	// BatchSize is a fetch batch size
	BatchSize int `mapstructure:"batch"`

	// Namespace is events namespace
	Namespace string `mapstructure:"namespace"`

	// Types are event types to log
	Types []string `mapstructure:"types"`

	// StartTimeRaw is start time passed from CLI
	StartTimeRaw string `mapstructure:"start-time"`

	// StartTime is start time
	StartTime time.Time

	// Timeout is the time poller will wait before the new request if there are no events in the queue
	Timeout time.Duration
}

const (
	// envPrefix is the configuration environment prefix
	envPrefix = "FDF"

	// debug CLI flag name
	debug = "debug"
)

var (
	// defaultStartTime is time frame start used by default
	defaultStartTime = time.Now().AddDate(-5, 0, 0).UTC()
)

// initConfig initializes viper args
func initConfig() {
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()

	pflag.BoolP("help", "h", false, "Print usage message")

	pflag.StringP("teleport-addr", "p", "", "Teleport addr")
	pflag.StringP("teleport-identity", "i", "", "Teleport identity file")
	pflag.String("teleport-ca", "", "Teleport TLS CA file")
	pflag.String("teleport-cert", "", "Teleport TLS certificate file")
	pflag.String("teleport-key", "", "Teleport TLS key file")

	pflag.StringP("fluentd-url", "u", "", "fluentd url")
	pflag.StringP("fluentd-ca", "a", "", "fluentd TLS CA path")
	pflag.StringP("fluentd-cert", "c", "", "fluentd TLS certificate path")
	pflag.StringP("fluentd-key", "k", "", "fluentd TLS key path")

	pflag.StringP("storage-dir", "s", "", "Storage directory")
	pflag.Int("batch-size", 20, "Events API fetch batch size")
	pflag.String("namespace", "default", "Events namespace")
	pflag.StringSliceP("types", "t", []string{}, "Event types to log")
	pflag.String("start-time", defaultStartTime.Format(time.RFC3339), "Start time to fetch events from in RFC3339 format")
	pflag.Duration("timeout", 5*time.Second, "Polling timeout")

	pflag.BoolP(debug, "d", false, "Debug mode")

	pflag.Parse()

	viper.BindPFlags(pflag.CommandLine)

	if viper.GetBool(debug) {
		log.SetLevel(log.DebugLevel)
	}

	//https://stackoverflow.com/questions/56129533/tls-with-certificate-private-key-and-pass-phrase
	//pflag.StringP(FluentdPassphrase, "p", "", "fluentd key passphrase")
}

// printUsage calls respective pflag method which prints usage message
func printUsage() {
	fmt.Println()
	pflag.PrintDefaults()
}

// newConfig creates new config structб fills it in from CLI and validates that required args are present
func newConfig() (*Config, error) {
	c := &Config{}
	err := viper.Unmarshal(c)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = c.validate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return c, nil
}

// Validate validates that required CLI args are present
func (c *Config) validate() error {
	err := c.validateFluentd()
	if err != nil {
		return err
	}

	err = c.validateTeleport()
	if err != nil {
		return err
	}

	err = c.validateStorage()
	if err != nil {
		return err
	}

	c.StartTime, err = time.Parse(time.RFC3339, c.StartTimeRaw)
	if err != nil {
		return trace.Wrap(err)
	}

	log.WithFields(log.Fields{"dir": c.StorageDir}).Debug("Using storage dir")
	log.WithFields(log.Fields{"batch": c.BatchSize}).Debug("Using batch size")
	log.WithFields(log.Fields{"namespace": c.Namespace}).Debug("Using namespace")
	log.WithFields(log.Fields{"types": c.Types}).Debug("Using type filter")
	log.WithFields(log.Fields{"value": c.StartTime}).Debug("Using start time")
	log.WithFields(log.Fields{"timeout": c.Timeout}).Debug("Using timeout")

	return nil
}

// validateFluentd validates Fluentd CLI args
func (c *Config) validateFluentd() error {
	if c.FluentdURL == "" {
		return trace.BadParameter("Pass fluentd url")
	}

	if c.FluentdCA != "" && !fileExists(c.FluentdCA) {
		return trace.BadParameter("Fluentd CA file does not exist %s", c.FluentdCA)
	}

	if c.FluentdCert == "" {
		return trace.BadParameter("HTTPS must be enabled in fluentd. Please, specify fluentd TLS certificate")
	}

	if !fileExists(c.FluentdCert) {
		return trace.BadParameter("Fluentd certificate file does not exist %s", c.FluentdCert)
	}

	if c.FluentdKey == "" {
		return trace.BadParameter("HTTPS must be enabled in fluentd. Please, specify fluentd TLS key")
	}

	if !fileExists(c.FluentdKey) {
		return trace.BadParameter("Fluentd key file does not exist %s", c.FluentdKey)
	}

	log.WithFields(log.Fields{"url": c.FluentdURL}).Debug("Using Fluentd url")
	log.WithFields(log.Fields{"ca": c.FluentdCA}).Debug("Using Fluentd ca")
	log.WithFields(log.Fields{"cert": c.FluentdCert}).Debug("Using Fluentd cert")
	log.WithFields(log.Fields{"key": c.FluentdKey}).Debug("Using Fluentd key")

	return nil
}

// validateTeleport validates Teleport CLI args
func (c *Config) validateTeleport() error {
	// If any of key files are specified
	if c.TeleportCA != "" || c.TeleportCert != "" || c.TeleportKey != "" {
		// Then addr becomes required
		if c.TeleportAddr == "" {
			return trace.BadParameter("Please, specify Teleport address using")
		}

		// And all of the files must be specified
		if c.TeleportCA == "" {
			return trace.BadParameter("Please, provide Teleport TLS CA")
		}

		if !fileExists(c.TeleportCA) {
			return trace.BadParameter("Teleport TLS CA file does not exist %s", c.TeleportCA)
		}

		if c.TeleportCert == "" {
			return trace.BadParameter("Please, provide Teleport TLS certificate")
		}

		if !fileExists(c.TeleportCert) {
			return trace.BadParameter("Teleport TLS certificate file does not exist %s", c.TeleportCert)
		}

		if c.TeleportKey == "" {
			return trace.BadParameter("Please, provide Teleport TLS key")
		}

		if !fileExists(c.TeleportKey) {
			return trace.BadParameter("Teleport TLS key file does not exist %s", c.TeleportKey)
		}

		log.WithFields(log.Fields{"addr": c.TeleportAddr}).Debug("Using Teleport addr")
		log.WithFields(log.Fields{"ca": c.TeleportCA}).Debug("Using Teleport CA")
		log.WithFields(log.Fields{"cert": c.TeleportCert}).Debug("Using Teleport cert")
		log.WithFields(log.Fields{"key": c.TeleportKey}).Debug("Using Teleport key")
	} else {
		if c.TeleportIdentityFile == "" {
			// Otherwise, we need identity file
			return trace.BadParameter("Please, specify either identity file or certificates to connect to Teleport")
		}

		if !fileExists(c.TeleportIdentityFile) {
			return trace.BadParameter("Teleport identity file does not exist %s", c.TeleportIdentityFile)
		}
	}

	return nil
}

func (c *Config) validateStorage() error {
	if c.StorageDir == "" {
		return trace.BadParameter("Storage dir is empty, pass storage dir")
	}

	_, err := os.Stat(c.StorageDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(c.StorageDir, 0755)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// fileExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
