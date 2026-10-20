package xnode

import (
	"crypto/sha256"
	"encoding/json"
	"sync"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/VanSail-Opensource/X-Node/pkg/xrayconfig"
	"github.com/xtls/xray-core/core"
)

// validationCache retains at most one successful candidate per context, with a
// bounded number of contexts. Native/synthetic documents are never persisted.
type validationCache struct {
	mu      sync.Mutex
	clock   uint64
	entries map[string]*validationEntry
}

type validationEntry struct {
	mu        sync.Mutex
	used      uint64 // protected by validationCache.mu
	key       [32]byte
	valid     bool
	result    ValidationResult
	validator xrayconfig.Validator
}

func (c *validationCache) entry(context string) *validationEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]*validationEntry{}
	}
	c.clock++
	entry := c.entries[context]
	if entry == nil {
		if len(c.entries) >= 128 {
			oldest := ""
			var used uint64
			for k, v := range c.entries {
				if oldest == "" || v.used < used {
					oldest, used = k, v.used
				}
			}
			delete(c.entries, oldest)
		}
		entry = &validationEntry{}
		c.entries[context] = entry
	}
	entry.used = c.clock
	return entry
}

func (s *Service) validateCompiledCached(node string, config wire.Config) ValidationResult {
	if err := config.Validate(); err != nil {
		return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID", Detail: validationDetail(err)}
	}
	if s.compilation == nil {
		return ValidateCompiled(config)
	}
	// Keep declarations that require real node validation in the proof key;
	// normalized/synthetic documents alone cannot prove those dependencies.
	var native any
	if err := json.Unmarshal(config.Xray, &native); err != nil {
		return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
	}
	raw, _ := json.Marshal([]any{core.Version(), json.RawMessage(wire.NativeIdentity(config)), native})
	key := sha256.Sum256(raw)
	entry := s.compilation.entry("node:" + node)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.valid && entry.key == key {
		return entry.result
	}
	result := validateCompiled(config, &entry.validator)
	if result.Valid {
		entry.key, entry.valid, entry.result = key, true, result
	}
	return result
}

func (s *Service) validateDocumentCached(context string, raw json.RawMessage) ValidationResult {
	if s.compilation == nil {
		return ValidateConfigurationDocument(raw)
	}
	var native any
	if json.Unmarshal(raw, &native) != nil {
		return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
	}
	canonical, err := json.Marshal(native)
	if err != nil {
		return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
	}
	key := sha256.Sum256(append([]byte(core.Version()+"\n"), canonical...))
	entry := s.compilation.entry("document:" + context)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.valid && entry.key == key {
		return entry.result
	}
	result := validateConfigurationDocument(raw, &entry.validator)
	if result.Valid {
		entry.key, entry.valid, entry.result = key, true, result
	}
	return result
}

func nativeValidate(config wire.Config, validator *xrayconfig.Validator) error {
	if validator != nil {
		return validator.Validate(config)
	}
	return xrayconfig.Validate(config)
}

func compiledFingerprint(config wire.Config) [32]byte {
	// Connection-derived controls have their own cheap synchronization frames.
	config.Users = append([]wire.User(nil), config.Users...)
	for i := range config.Users {
		config.Users[i].BlockNew = false
	}
	raw, _ := json.Marshal(config)
	return sha256.Sum256(raw)
}
