package whatsapp

import "time"

type GroupInfo struct {
	JID              string `json:"jid"`
	Name             string `json:"name"`
	Topic            string `json:"topic,omitempty"`
	ParticipantCount int    `json:"participant_count"`
}

type ContactInfo struct {
	JID   string `json:"jid"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
}

type SessionSyncData struct {
	NumberID string        `json:"number_id"`
	Phone    string        `json:"phone,omitempty"`
	Contacts []ContactInfo `json:"contacts"`
	Groups   []GroupInfo   `json:"groups"`
	Error    string        `json:"error,omitempty"`
}

type AllSyncResult struct {
	TotalSessions int               `json:"total_sessions"`
	TotalContacts int               `json:"total_contacts"`
	TotalGroups   int               `json:"total_groups"`
	Results       []SessionSyncData `json:"results"`
}

type InboundMessage struct {
	NumberID   string
	SenderJID  string
	SenderName string
	ChatJID    string
	Text       string
	Timestamp  time.Time
	IsGroup    bool
	// IsFromMe marks a message the account owner sent from another device (the phone). SenderJID is
	// then the owner, and ChatJID is the person or group it was sent to.
	IsFromMe  bool
	MessageID string
}

type StatusChange struct {
	NumberID string
	Status   string
}

// SessionInfo describes a WhatsApp device session known to the daemon,
// whether it is currently loaded in memory or only present on disk.
type SessionInfo struct {
	NumberID   string `json:"number_id"`
	Connected  bool   `json:"connected"`
	HasFile    bool   `json:"has_file"`
	Paired     bool   `json:"paired"`
	PairingKey string `json:"pairing_key,omitempty"`
}
