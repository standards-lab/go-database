package database

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/standards-lab/go-core/config"
)

const (
	defaultHost            = "localhost"
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 15 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
	defaultConnTimeout     = 5 * time.Second
)

// Config holds the connection identity, pool sizing, and timeouts for a SQL
// database. The numeric and duration fields are tri-state pointers: nil is
// unset and takes the default, while an explicit zero survives the load and
// means what it says — an unlimited pool, or no idle connections. Port has no
// default here because the default port is a provider fact; each provider
// supplies its own. User and Password are optional because their requiredness
// varies by provider and auth mode; the password rides the secrets layer of
// [config.Load] rather than a committed file. Options carries dialect-specific
// connection keys (postgres: sslmode) passed through to the provider. Env
// records the environment-variable names Finalize composed and read; it is
// excluded from JSON.
type Config struct {
	Host            string            `json:"host"`
	Name            string            `json:"name"`
	User            string            `json:"user"`
	Password        string            `json:"password"`
	Port            *int              `json:"port"`
	MaxOpenConns    *int              `json:"max_open_conns"`
	MaxIdleConns    *int              `json:"max_idle_conns"`
	ConnMaxLifetime *config.Duration  `json:"conn_max_lifetime"`
	ConnMaxIdleTime *config.Duration  `json:"conn_max_idle_time"`
	ConnTimeout     *config.Duration  `json:"conn_timeout"`
	Options         map[string]string `json:"options"`
	Env             Env               `json:"-"`
}

// Merge overlays src's set fields onto the receiver. Options merges key-wise,
// so an overlay can set one connection option without dropping the rest.
func (c *Config) Merge(src *Config) {
	if src.Host != "" {
		c.Host = src.Host
	}
	if src.Name != "" {
		c.Name = src.Name
	}
	if src.User != "" {
		c.User = src.User
	}
	if src.Password != "" {
		c.Password = src.Password
	}
	if src.Port != nil {
		c.Port = src.Port
	}
	if src.MaxOpenConns != nil {
		c.MaxOpenConns = src.MaxOpenConns
	}
	if src.MaxIdleConns != nil {
		c.MaxIdleConns = src.MaxIdleConns
	}
	if src.ConnMaxLifetime != nil {
		c.ConnMaxLifetime = src.ConnMaxLifetime
	}
	if src.ConnMaxIdleTime != nil {
		c.ConnMaxIdleTime = src.ConnMaxIdleTime
	}
	if src.ConnTimeout != nil {
		c.ConnTimeout = src.ConnTimeout
	}

	for k, v := range src.Options {
		if c.Options == nil {
			c.Options = make(map[string]string, len(src.Options))
		}
		c.Options[k] = v
	}
}

// Finalize composes the environment override names from envPrefix (an empty
// prefix disables overrides), applies defaults, applies the overrides, and
// validates. Name is the one required field; a malformed override fails with
// an error naming its variable.
func (c *Config) Finalize(envPrefix string) error {
	c.Env = NewEnv(envPrefix)
	c.applyDefaults()
	if err := c.applyEnv(); err != nil {
		return err
	}
	return c.validate()
}

func (c *Config) applyDefaults() {
	if c.Host == "" {
		c.Host = defaultHost
	}
	if c.MaxOpenConns == nil {
		c.MaxOpenConns = new(defaultMaxOpenConns)
	}
	if c.MaxIdleConns == nil {
		c.MaxIdleConns = new(defaultMaxIdleConns)
	}
	if c.ConnMaxLifetime == nil {
		c.ConnMaxLifetime = new(config.Duration(defaultConnMaxLifetime))
	}
	if c.ConnMaxIdleTime == nil {
		c.ConnMaxIdleTime = new(config.Duration(defaultConnMaxIdleTime))
	}
	if c.ConnTimeout == nil {
		c.ConnTimeout = new(config.Duration(defaultConnTimeout))
	}
}

func (c *Config) applyEnv() error {
	if v := os.Getenv(c.Env.Host); v != "" {
		c.Host = v
	}
	if v := os.Getenv(c.Env.Name); v != "" {
		c.Name = v
	}
	if v := os.Getenv(c.Env.User); v != "" {
		c.User = v
	}
	if v := os.Getenv(c.Env.Password); v != "" {
		c.Password = v
	}
	if v := os.Getenv(c.Env.Port); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Env.Port, err)
		}
		c.Port = &port
	}
	if v := os.Getenv(c.Env.MaxOpenConns); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Env.MaxOpenConns, err)
		}
		c.MaxOpenConns = &n
	}
	if v := os.Getenv(c.Env.MaxIdleConns); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Env.MaxIdleConns, err)
		}
		c.MaxIdleConns = &n
	}
	if err := config.SetDurationFromEnv(&c.ConnMaxLifetime, c.Env.ConnMaxLifetime); err != nil {
		return err
	}
	if err := config.SetDurationFromEnv(&c.ConnMaxIdleTime, c.Env.ConnMaxIdleTime); err != nil {
		return err
	}
	return config.SetDurationFromEnv(&c.ConnTimeout, c.Env.ConnTimeout)
}

func (c *Config) validate() error {
	if c.Name == "" {
		return errors.New("database name required")
	}
	if c.Port != nil && (*c.Port < 1 || *c.Port > 65535) {
		return fmt.Errorf("invalid port: %d", *c.Port)
	}
	if *c.MaxOpenConns < 0 {
		return fmt.Errorf("invalid max_open_conns: %d", *c.MaxOpenConns)
	}
	if *c.MaxIdleConns < 0 {
		return fmt.Errorf("invalid max_idle_conns: %d", *c.MaxIdleConns)
	}
	if *c.MaxOpenConns > 0 && *c.MaxIdleConns > *c.MaxOpenConns {
		return fmt.Errorf(
			"max_idle_conns (%d) must not exceed max_open_conns (%d)",
			*c.MaxIdleConns, *c.MaxOpenConns,
		)
	}
	if *c.ConnMaxLifetime < 0 {
		return fmt.Errorf("invalid conn_max_lifetime: %s", c.ConnMaxLifetime)
	}
	if *c.ConnMaxIdleTime < 0 {
		return fmt.Errorf("invalid conn_max_idle_time: %s", c.ConnMaxIdleTime)
	}
	if *c.ConnTimeout <= 0 {
		return fmt.Errorf("conn_timeout must be positive, got %s", c.ConnTimeout)
	}
	return nil
}

func (c *Config) finalized() bool {
	return c.MaxOpenConns != nil &&
		c.MaxIdleConns != nil &&
		c.ConnMaxLifetime != nil &&
		c.ConnMaxIdleTime != nil &&
		c.ConnTimeout != nil
}
