// Package xnode owns the panel's authoritative X-Node control plane.
package xnode

import (
	"encoding/json"
	"errors"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const MultiplierScale int64 = 1_000_000
const GlobalSettingKey = "xnode.global"

var ErrNotFound = errors.New("X-Node resource not found")
var ErrConflict = errors.New("X-Node revision conflict")
var ErrUnavailable = errors.New("X-Node agent unavailable")
var ErrInvalid = errors.New("invalid X-Node input")

type Node struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	CountryCode      string    `json:"countryCode"`
	ProviderName     string    `json:"providerName"`
	MultiplierFP     int64     `json:"-"`
	Multiplier       float64   `json:"multiplier"`
	Enabled          bool      `json:"enabled"`
	ConfigID         string    `json:"configId"`
	SettingsCipher   string    `json:"-"`
	TokenHash        []byte    `json:"-"`
	AgentID          string    `json:"agentId"`
	DesiredRevision  uint64    `json:"desiredRevision"`
	AcceptedRevision uint64    `json:"acceptedRevision"`
	AppliedRevision  uint64    `json:"appliedRevision"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Configuration struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	DocumentCipher string          `json:"-"`
	Document       json.RawMessage `json:"document,omitempty"`
	SnippetIDs     []string        `json:"snippetIds"`
	Revision       int64           `json:"revision"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type Snippet struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	DocumentCipher string          `json:"-"`
	Document       json.RawMessage `json:"document,omitempty"`
	Revision       int64           `json:"revision"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type GlobalConfig struct {
	FirewallDetection      *FirewallDetectionConfig `json:"firewallDetection"`
	GeodataIntervalHours   *int                     `json:"geodataIntervalHours,omitempty"`
	SourceGeoIP            SourceGeoIPConfig        `json:"sourceGeoIp"`
	BlockedDomains         []string                 `json:"blockedDomains"`
	FirewallUnbanPrice     string                   `json:"firewallUnbanPrice"`
	FirewallExemptionHours int                      `json:"firewallExemptionHours"`
	ConnectionLimit        int                      `json:"connectionLimit"`
	BlockedPorts           string                   `json:"blockedPorts"`
	GeoIPURL               string                   `json:"geoIpUrl"`
	GeoSiteURL             string                   `json:"geoSiteUrl"`
	GeoIPHash              string                   `json:"geoIpHash"`
	GeoSiteHash            string                   `json:"geoSiteHash"`
	GeodataCron            string                   `json:"geodataCron"`
	Torrent                wire.TorrentPolicy       `json:"torrent"`
	Telemetry              wire.TelemetryPolicy     `json:"telemetry"`
	Updates                wire.UpdatePolicy        `json:"updates"`
	Providers              map[string]wire.Provider `json:"providers"`
	SpeedtestDownloadURL   string                   `json:"speedtestDownloadUrl"`
	SpeedtestUploadURL     string                   `json:"speedtestUploadUrl"`
}

type NodeSettings struct {
	DDNSBindings        map[string]DDNSHostBinding `json:"ddnsBindings,omitempty"`
	ddnsActor           string
	Bandwidth           *BandwidthPolicy            `json:"bandwidth,omitempty"`
	DDNS                []wire.DDNSJob              `json:"ddns"`
	Certificates        map[string]wire.Certificate `json:"certificates"`
	InboundCertificates map[string]string           `json:"inboundCertificates"`
	DNS                 wire.DNSPolicy              `json:"dns"`
	Firewall            wire.FirewallPolicy         `json:"firewall"`
}

type Group struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Enabled  bool           `json:"enabled"`
	Bindings []GroupBinding `json:"bindings"`
}
type GroupBinding struct {
	GroupID    string `json:"groupId"`
	NodeID     string `json:"nodeId"`
	InboundTag string `json:"inboundTag"`
}

type Endpoint struct {
	ID              string          `json:"id"`
	NodeID          string          `json:"nodeId"`
	InboundTag      string          `json:"inboundTag"`
	Name            string          `json:"name"`
	Address         string          `json:"address"`
	Port            int             `json:"port"`
	Enabled         bool            `json:"enabled"`
	Hidden          bool            `json:"hidden"`
	OverridesCipher string          `json:"-"`
	Overrides       json.RawMessage `json:"overrides,omitempty"`
}
