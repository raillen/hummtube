package domain

type SessionPersistence string
type CredentialState string

const (
	SessionPersistenceKeyring  SessionPersistence = "keyring"
	SessionPersistenceMemory   SessionPersistence = "memory"
	CredentialStateAvailable   CredentialState    = "available"
	CredentialStateMemory      CredentialState    = "memory"
	CredentialStateUnavailable CredentialState    = "unavailable"
	CredentialStateMissing     CredentialState    = "missing"
)

// AccountInfo is the non-sensitive account state kept locally.
// Sensitive tokens never enter this DTO or SQLite. They remain in the
// SecretStore, or only in process memory during an explicit fallback.
type AccountInfo struct {
	ProfileID          string             `json:"profile_id"`
	Provider           string             `json:"provider"`
	ProviderSubject    string             `json:"provider_subject"`
	Email              string             `json:"email"`
	ConnectedAt        string             `json:"connected_at"` // RFC3339
	SessionPersistence SessionPersistence `json:"session_persistence"`
	CredentialState    CredentialState    `json:"credential_state"`
	Warning            string             `json:"warning,omitempty"`
}
