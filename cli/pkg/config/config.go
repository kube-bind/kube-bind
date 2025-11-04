/*
Copyright 2025 The Kube Bind Authors.

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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
)

// Config represents the kube-bind CLI configuration
type Config struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Servers    map[string]*Server `json:"servers,omitempty"`
	Current    string             `json:"current,omitempty"`

	// configFile is the path to the kube-bind configuration file
	configFile string `json:"-"`
}

// Server represents authentication details for a kube-bind server
type Server struct {
	URL         string    `json:"url"`
	Cluster     string    `json:"cluster,omitempty"`
	AccessToken string    `json:"accessToken,omitempty"`
	TokenType   string    `json:"tokenType,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
	Username    string    `json:"username,omitempty"`
}

// NewConfig creates a new empty configuration
func NewConfig(configFile string) *Config {
	// if configFile is empty, use default location
	if configFile == "" {
		configFile = GetDefaultConfigFilePath()
	}
	return &Config{
		APIVersion: "v1",
		Kind:       "Config",
		Servers:    make(map[string]*Server),
		configFile: configFile,
	}
}

// GetDefaultConfigFilePath returns the default path to the kube-bind config file
func GetDefaultConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path.Join(".", "config")
	}
	return path.Join(homeDir, ".kube-bind", "config")
}

// GetConfigPath returns the path to the kube-bind config file
func (c *Config) GetConfigPath() (string, error) {
	return c.configFile, nil
}

// LoadConfig loads the kube-bind configuration from file
func LoadConfigFromFile(configFile string) (*Config, error) {
	// If config file doesn't exist, return empty config
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return NewConfig(configFile), nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to parse config file: %w", err)
	}
	config.configFile = configFile

	// Initialize servers map if nil
	if config.Servers == nil {
		config.Servers = make(map[string]*Server)
	}

	return &config, nil
}

// SaveConfig saves the kube-bind configuration to file
func (c *Config) SaveConfig() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal config: %w", err)
	}

	configFile, err := c.GetConfigPath()
	if err != nil {
		return fmt.Errorf("unable to get config file path: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("unable to write config file %s: %w", configFile, err)
	}

	return nil
}

func (c *Config) Get(serverURL, clusterID string) (*Server, bool) {
	key := buildServerKey(serverURL, clusterID)
	server, exists := c.Servers[key]
	return server, exists
}

// AddServerWithCluster adds or updates server configuration with cluster ID
func (c *Config) AddServerWithCluster(serverURL, clusterID string, server *Server) {
	key := buildServerKey(serverURL, clusterID)
	c.addServer(key, server)
}

// addServer adds or updates server configuration
func (c *Config) addServer(name string, server *Server) {
	if c.Servers == nil {
		c.Servers = make(map[string]*Server)
	}
	c.Servers[name] = server
	c.Current = name
}

// FindServersByURL finds all servers for a given URL (across all clusters)
func (c *Config) FindServersByURL(serverURL string) map[string]*Server {
	matches := make(map[string]*Server)
	for key, server := range c.Servers {
		keyServerURL, _ := parseServerKey(key)
		if keyServerURL == serverURL {
			matches[key] = server
		}
	}
	return matches
}

// RemoveServer removes a server from the configuration
func (c *Config) RemoveServer(name, cluster string) {
	delete(c.Servers, buildServerKey(name, cluster))
	if c.Current == name {
		c.Current = ""
		// Set current to first available server
		for serverName := range c.Servers {
			c.Current = serverName
			break
		}
	}
}

// GetCurrentServer returns the currently active server configuration.
func (c *Config) GetCurrentServer() (*Server, string, error) {
	if c.Current == "" {
		return nil, "", fmt.Errorf("no current server set")
	}

	server, exists := c.Servers[c.Current]
	if !exists {
		return nil, "", fmt.Errorf("current server %q not found in config", c.Current)
	}

	return server, c.Current, nil
}

// SetCurrentServer sets the current active server
func (c *Config) SetCurrentServer(serverName, cluster string) error {
	if _, exists := c.Servers[buildServerKey(serverName, cluster)]; !exists {
		return fmt.Errorf("server %q does not exist", serverName)
	}

	c.Current = buildServerKey(serverName, cluster)
	return nil
}

// buildServerKey creates a unique key for server+cluster combination
func buildServerKey(serverURL, clusterID string) string {
	if clusterID == "" {
		return serverURL
	}

	// Use @ separator to combine server and cluster (similar to user@host pattern)
	return fmt.Sprintf("%s@%s", clusterID, serverURL)
}

// parseServerKey parses a server key back into serverURL and clusterID.
func parseServerKey(key string) (serverURL, clusterID string) {
	if parts := strings.SplitN(key, "@", 2); len(parts) == 2 {
		return parts[1], parts[0]
	}
	return key, ""
}
