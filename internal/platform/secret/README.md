# Secret vault

- `vault.go` encrypts and decrypts setting-bound secrets using versioned authenticated encryption.
- `vault_test.go` verifies round trips, setting-name binding, key validation, and malformed ciphertext rejection.
- `connection_ip.go` opens retained encrypted IP targets with their canonical
  owner/node/digest context first. Its bounded fallback accepts only exact old
  owners from retained migration evidence; it never resolves native principals,
  rewrites ciphertext or grants authorization. `connection_ip_test.go` covers
  foreign owners, changed targets, capacity and cancellation fences.
