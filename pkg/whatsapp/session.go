package whatsapp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
)

type Manager struct {
	SessionDir     string
	OnMessage      func(msg InboundMessage)
	OnStatusChange func(status StatusChange)

	mu       sync.RWMutex
	sessions map[string]*Session
}

type Session struct {
	Client      *whatsmeow.Client
	Container   *sqlstore.Container
	DBPath      string // actual .db file path on disk
	NumberID    string // current session key (may be updated after QR pairing)
	QR          string
	PairingKey  string // unique ownership token shown in UI, reported to central backend
	mu          sync.RWMutex
}

func NewManager(sessionDir string) *Manager {
	return &Manager{
		SessionDir: sessionDir,
		sessions:   make(map[string]*Session),
	}
}

func (m *Manager) initClient(numberID string) (*whatsmeow.Client, *sqlstore.Container, error) {
	dbPath := filepath.Join(m.SessionDir, fmt.Sprintf("%s.db", numberID))
	dbLog := waLog.Noop // or waLog.Stdout("Database", "ERROR", true)
	
	if err := os.MkdirAll(m.SessionDir, 0755); err != nil {
		return nil, nil, err
	}

	container, err := sqlstore.New(context.Background(), "sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", dbPath), dbLog)
	if err != nil {
		return nil, nil, err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		container.Close()
		return nil, nil, err
	}

	clientLog := waLog.Noop
	client := whatsmeow.NewClient(deviceStore, clientLog)
	return client, container, nil
}

func (m *Manager) StartPairing(numberID string) error {
	m.mu.Lock()
	if s, exists := m.sessions[numberID]; exists {
		if s.Client != nil && s.Client.IsConnected() {
			m.mu.Unlock()
			return fmt.Errorf("session already connected for %s", numberID)
		}
		// Stale half-paired session — tear down so a fresh QR can be issued.
		if s.Client != nil {
			s.Client.Disconnect()
		}
		if s.Container != nil {
			s.Container.Close()
		}
		delete(m.sessions, numberID)
	}
	m.mu.Unlock()

	// Drop any on-disk session for this number so pairing starts from a clean
	// device store. A half-paired .db has Store.ID != nil, which makes
	// GetQRChannel never emit a code and leaves QR permanently empty.
	dbPath := filepath.Join(m.SessionDir, numberID+".db")
	_ = os.Remove(dbPath)

	client, container, err := m.initClient(numberID)
	if err != nil {
		return err
	}

	session := &Session{
		Client:    client,
		Container: container,
		DBPath:    dbPath,
		NumberID:  numberID,
	}
	m.mu.Lock()
	m.sessions[numberID] = session
	m.mu.Unlock()

	// Use session.NumberID so event handler tracks renames after QR pairing.
	client.AddEventHandler(func(evt interface{}) {
		session.mu.RLock()
		id := session.NumberID
		session.mu.RUnlock()
		m.handleEvent(id, evt)
	})

	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		m.Stop(numberID, true)
		return err
	}
	if err := client.Connect(); err != nil {
		m.Stop(numberID, true)
		return err
	}

	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				png, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
				if err == nil {
					base64PNG := base64.StdEncoding.EncodeToString(png)
					session.mu.Lock()
					session.QR = fmt.Sprintf("data:image/png;base64,%s", base64PNG)
					session.mu.Unlock()
				}
			} else {
				session.mu.Lock()
				session.QR = ""
				session.mu.Unlock()
			}
		}
	}()

	return nil
}

// PairWithCode links a personal number using an 8-digit pairing code instead of
// a QR scan. Returns the code the admin enters on their phone.
func (m *Manager) PairWithCode(numberID, phone string) (string, error) {
	m.mu.Lock()
	if _, exists := m.sessions[numberID]; exists {
		m.mu.Unlock()
		return "", fmt.Errorf("session already exists for %s", numberID)
	}
	m.mu.Unlock()

	client, container, err := m.initClient(numberID)
	if err != nil {
		return "", err
	}

	dbPath := filepath.Join(m.SessionDir, numberID+".db")
	session := &Session{Client: client, Container: container, DBPath: dbPath, NumberID: numberID}
	m.mu.Lock()
	m.sessions[numberID] = session
	m.mu.Unlock()

	client.AddEventHandler(func(evt interface{}) {
		session.mu.RLock()
		id := session.NumberID
		session.mu.RUnlock()
		m.handleEvent(id, evt)
	})

	if err := client.Connect(); err != nil {
		m.Stop(numberID, true)
		return "", err
	}

	code, err := client.PairPhone(context.Background(), phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		m.Stop(numberID, true)
		return "", err
	}
	return code, nil
}

// DeviceJID returns the paired device JID for a session, or "" if unpaired.
func (m *Manager) DeviceJID(numberID string) string {
	m.mu.RLock()
	s, ok := m.sessions[numberID]
	m.mu.RUnlock()
	if !ok || s.Client == nil || s.Client.Store.ID == nil {
		return ""
	}
	return s.Client.Store.ID.ToNonAD().String()
}

// pairingKeyAlphabet excludes ambiguous characters (0/O, 1/I) so a
// hand-typed pairing key is never misread.
const pairingKeyAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generatePairingKey() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is catastrophic; fall back to a time-seeded value
		// so a device is never left without a key.
		return fmt.Sprintf("PAIR-%04d-%04d", time.Now().UnixMilli()%10000, time.Now().UnixNano()%10000)
	}
	chars := make([]byte, 8)
	for i, b := range buf {
		chars[i] = pairingKeyAlphabet[int(b)%len(pairingKeyAlphabet)]
	}
	return fmt.Sprintf("PAIR-%s-%s", chars[:4], chars[4:])
}

func (m *Manager) pairingKeyPath(numberID string) string {
	return filepath.Join(m.SessionDir, fmt.Sprintf("%s.key", numberID))
}

func (m *Manager) savePairingKey(numberID, key string) {
	if key == "" {
		return
	}
	if err := os.MkdirAll(m.SessionDir, 0755); err != nil {
		log.Printf("[SessionManager] mkdir for pairing key %s: %v", numberID, err)
		return
	}
	if err := os.WriteFile(m.pairingKeyPath(numberID), []byte(key+"\n"), 0600); err != nil {
		log.Printf("[SessionManager] save pairing key %s: %v", numberID, err)
	}
}

func (m *Manager) loadPairingKey(numberID string) string {
	b, err := os.ReadFile(m.pairingKeyPath(numberID))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (m *Manager) removePairingKey(numberID string) {
	if err := os.Remove(m.pairingKeyPath(numberID)); err != nil && !os.IsNotExist(err) {
		log.Printf("[SessionManager] remove pairing key %s: %v", numberID, err)
	}
}

// PairingKey returns the unique ownership token for a session, or "" if the
// session is not loaded or has never connected.
func (m *Manager) PairingKey(numberID string) string {
	m.mu.RLock()
	s, ok := m.sessions[numberID]
	m.mu.RUnlock()
	if !ok {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PairingKey
}

// resolvePN maps a LID address to the person's phone-number JID using the alternate address
// WhatsApp supplied, or the local LID map. Anything else is returned unchanged.
func resolvePN(client *whatsmeow.Client, jid, alt types.JID) types.JID {
	if jid.Server != types.HiddenUserServer {
		return jid
	}
	if alt.Server == types.DefaultUserServer && !alt.IsEmpty() {
		return alt
	}
	if client != nil && client.Store != nil && client.Store.LIDs != nil {
		if pn, err := client.Store.LIDs.GetPNForLID(context.Background(), jid); err == nil && !pn.IsEmpty() {
			return pn
		}
	}
	return jid
}

func (m *Manager) handleEvent(numberID string, evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if m.OnMessage != nil {
			isGroup := v.Info.IsGroup
			text := ""
			if v.Message.GetConversation() != "" {
				text = v.Message.GetConversation()
			} else if v.Message.GetExtendedTextMessage() != nil {
				text = v.Message.GetExtendedTextMessage().GetText()
			}

			if text != "" {
				// WhatsApp may address people by an anonymous LID instead of their phone number.
				// Report phone-number JIDs so contacts and replies line up with real numbers.
				var client *whatsmeow.Client
				if sess := m.findSession(numberID); sess != nil {
					client = sess.Client
				}
				chatAlt := v.Info.SenderAlt
				if v.Info.IsFromMe {
					chatAlt = v.Info.RecipientAlt
				}
				m.OnMessage(InboundMessage{
					NumberID:   numberID,
					SenderJID:  resolvePN(client, v.Info.Sender, v.Info.SenderAlt).ToNonAD().String(),
					SenderName: v.Info.PushName,
					ChatJID:    resolvePN(client, v.Info.Chat, chatAlt).ToNonAD().String(),
					Text:       text,
					Timestamp:  v.Info.Timestamp,
					IsGroup:    isGroup,
					IsFromMe:   v.Info.IsFromMe,
					MessageID:  v.Info.ID,
				})
			}
		}
	case *events.PairSuccess:
		phone := v.ID.User
		if phone != "" {
			m.mu.Lock()
			if session, ok := m.sessions[numberID]; ok {
				session.mu.Lock()
				session.NumberID = phone
				session.QR = ""
				session.mu.Unlock()
				m.sessions[phone] = session
			}
			m.mu.Unlock()
		}
	case *events.Connected:
		var notifyID = numberID
		m.mu.Lock()
		if session, ok := m.sessions[numberID]; ok {
			session.mu.Lock()
			session.QR = ""
			if session.Client != nil && session.Client.Store.ID != nil && session.Client.Store.ID.User != "" {
				phone := session.Client.Store.ID.User
				session.NumberID = phone
				// A device gets exactly one pairing key for its lifetime. Generate
				// it idempotently on connect so both fresh pairs and restarts work.
				if session.PairingKey == "" {
					session.PairingKey = generatePairingKey()
					m.savePairingKey(phone, session.PairingKey)
				}
				m.sessions[phone] = session
				notifyID = phone
			}
			session.mu.Unlock()
		}
		m.mu.Unlock()

		if m.OnStatusChange != nil {
			m.OnStatusChange(StatusChange{NumberID: notifyID, Status: "connected"})
			if notifyID != numberID {
				m.OnStatusChange(StatusChange{NumberID: numberID, Status: "connected"})
			}
		}
	case *events.Disconnected:
		if m.OnStatusChange != nil {
			m.OnStatusChange(StatusChange{NumberID: numberID, Status: "disconnected"})
		}
	case *events.LoggedOut:
		if m.OnStatusChange != nil {
			m.OnStatusChange(StatusChange{NumberID: numberID, Status: "logged_out"})
		}
		m.Stop(numberID, false)
	}
}

func (m *Manager) GetQR(numberID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[numberID]
	if !ok {
		return "", fmt.Errorf("session not found")
	}

	session.mu.RLock()
	qr := session.QR
	session.mu.RUnlock()

	if qr == "" {
		return "", fmt.Errorf("QR not available")
	}

	return qr, nil
}

func cleanNumber(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	return s
}

func (m *Manager) findSession(numberID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cleaned := cleanNumber(numberID)
	// 1. Exact match
	if s, ok := m.sessions[numberID]; ok && s != nil {
		return s
	}
	// 2. Cleaned match (without +)
	if cleaned != "" {
		if s, ok := m.sessions[cleaned]; ok && s != nil {
			return s
		}
	}
	// 3. Search through sessions
	for key, s := range m.sessions {
		if s == nil {
			continue
		}
		if key == numberID || key == cleaned || cleanNumber(key) == cleaned {
			return s
		}
		s.mu.RLock()
		nid := s.NumberID
		s.mu.RUnlock()
		if nid == numberID || nid == cleaned || cleanNumber(nid) == cleaned {
			return s
		}
		if s.Client != nil && s.Client.Store != nil && s.Client.Store.ID != nil {
			user := s.Client.Store.ID.User
			if user == numberID || user == cleaned || cleanNumber(user) == cleaned {
				return s
			}
		}
	}

	// 4. If numberID is empty or "all", or "default", return first connected session (or first session)
	if numberID == "" || numberID == "all" || strings.EqualFold(numberID, "default") || strings.EqualFold(numberID, "semua") {
		for _, s := range m.sessions {
			if s != nil && s.Client != nil && s.Client.IsConnected() {
				return s
			}
		}
		for _, s := range m.sessions {
			if s != nil {
				return s
			}
		}
	}

	// 5. If only 1 connected session exists in the manager, return that session as fallback
	var connectedSessions []*Session
	for _, s := range m.sessions {
		if s != nil && s.Client != nil && s.Client.IsConnected() {
			connectedSessions = append(connectedSessions, s)
		}
	}
	if len(connectedSessions) == 1 {
		return connectedSessions[0]
	}

	// 6. If only 1 session exists at all, return it
	if len(m.sessions) == 1 {
		for _, s := range m.sessions {
			if s != nil {
				return s
			}
		}
	}

	return nil
}

func (m *Manager) IsConnected(numberID string) bool {
	if session := m.findSession(numberID); session != nil && session.Client != nil {
		// IsLoggedIn only flips true after WhatsApp verifies the QR/pairing;
		// IsConnected is true the moment the socket opens to serve the QR, so it
		// must not be used to signal a successful pairing.
		return session.Client.IsLoggedIn()
	}
	return false
}

func (m *Manager) SendText(numberID, to, text string) error {
	session := m.findSession(numberID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Normalize a bare phone number: strip +, spaces, dashes, then append the
	// WhatsApp user domain. types.ParseJID("628...") does NOT error — it treats
	// the bare digits as the server name, so the domain must be attached before
	// parsing or SendMessage fails with "can't send message to unknown server".
	if !strings.Contains(to, "@") {
		to = strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, to)
		to += "@s.whatsapp.net"
	}

	jid, err := types.ParseJID(to)
	if err != nil {
		return fmt.Errorf("invalid recipient JID (%s): %w", to, err)
	}

	msg := &waE2E.Message{
		Conversation: &text,
	}
	_, err = session.Client.SendMessage(context.Background(), jid, msg)
	return err
}

func (m *Manager) SendGroupText(numberID, groupJID, text string) error {
	session := m.findSession(numberID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Bare group IDs also need their domain attached before parse (same
	// ParseJID pitfall as SendText).
	if !strings.Contains(groupJID, "@") {
		groupJID = strings.TrimSpace(groupJID) + "@g.us"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID (%s): %w", groupJID, err)
	}

	msg := &waE2E.Message{
		Conversation: &text,
	}
	_, err = session.Client.SendMessage(context.Background(), jid, msg)
	return err
}

func (m *Manager) ListGroups(numberID string) ([]GroupInfo, error) {
	if numberID == "" || numberID == "all" || strings.EqualFold(numberID, "semua") {
		return m.GetAllGroups()
	}

	session := m.findSession(numberID)
	if session == nil {
		all, err := m.GetAllGroups()
		if err == nil && len(all) > 0 {
			return all, nil
		}
		return nil, fmt.Errorf("session not found")
	}

	if session.Client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	groups, err := session.Client.GetJoinedGroups(context.Background())
	if err != nil {
		return nil, err
	}

	var infos []GroupInfo
	for _, g := range groups {
		infos = append(infos, GroupInfo{
			JID:              g.JID.String(),
			Name:             g.Name,
			Topic:            g.Topic,
			ParticipantCount: len(g.Participants),
		})
	}
	return infos, nil
}

func (m *Manager) ListContacts(numberID string) ([]ContactInfo, error) {
	if numberID == "" || numberID == "all" || strings.EqualFold(numberID, "semua") {
		return m.GetAllContacts()
	}

	session := m.findSession(numberID)
	if session == nil {
		all, err := m.GetAllContacts()
		if err == nil && len(all) > 0 {
			return all, nil
		}
		return nil, fmt.Errorf("session not found")
	}

	if session.Client == nil || session.Client.Store == nil || session.Client.Store.Contacts == nil {
		return nil, fmt.Errorf("session store not available")
	}

	contacts, err := session.Client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return nil, err
	}

	var infos []ContactInfo
	seen := make(map[string]int)
	for jid, c := range contacts {
		if jid.Server != types.DefaultUserServer {
			continue
		}
		phone := strings.Split(jid.User, ":")[0]
		if phone == "" {
			continue
		}
		name := c.FullName
		if name == "" {
			name = c.PushName
		}
		if name == "" {
			name = c.BusinessName
		}
		if name == "" {
			name = c.FirstName
		}
		if name == "" {
			name = "Kontak " + phone
		}
		if idx, exists := seen[phone]; exists {
			if strings.HasPrefix(infos[idx].Name, "Kontak ") && !strings.HasPrefix(name, "Kontak ") {
				infos[idx].Name = name
			}
			continue
		}
		seen[phone] = len(infos)
		infos = append(infos, ContactInfo{
			JID:   jid.String(),
			Phone: phone,
			Name:  name,
		})
	}
	return infos, nil
}

func (m *Manager) GetGroupMembers(numberID, groupJID string) ([]string, error) {
	session := m.findSession(numberID)
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	if !strings.Contains(groupJID, "@") {
		groupJID = strings.TrimSpace(groupJID) + "@g.us"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return nil, fmt.Errorf("invalid group JID (%s): %w", groupJID, err)
	}

	info, err := session.Client.GetGroupInfo(context.Background(), jid)
	if err != nil {
		return nil, err
	}

	var members []string
	for _, p := range info.Participants {
		members = append(members, p.JID.String())
	}
	return members, nil
}

func (m *Manager) Stop(numberID string, keepSession bool) error {
	m.mu.Lock()
	session, ok := m.sessions[numberID]
	var dbPath string
	if ok {
		dbPath = session.DBPath
		session.mu.RLock()
		altID := session.NumberID
		session.mu.RUnlock()
		delete(m.sessions, numberID)
		if altID != "" && altID != numberID {
			delete(m.sessions, altID)
		}
	}
	m.mu.Unlock()

	if dbPath == "" {
		dbPath = filepath.Join(m.SessionDir, fmt.Sprintf("%s.db", numberID))
	}

	if !ok {
		// Session not loaded in this process (disconnected, or never paired).
		// "Logout" still means: delete the on-disk session file so the device
		// stops reappearing after a restart.
		if keepSession {
			return fmt.Errorf("session not found")
		}
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		m.removePairingKey(numberID)
		return nil
	}

	session.Client.Disconnect()

	// Close the SQLite container to release the file lock before removal.
	if session.Container != nil {
		session.Container.Close()
	}

	if keepSession {
		return nil
	}

	// A live logout requires an active connection — a disconnected client can't
	// send the logout command, so skip it and just drop the on-disk session.
	var logoutErr error
	if session.Client.IsConnected() {
		logoutErr = session.Client.Logout(context.Background())
	}
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	m.removePairingKey(numberID)
	return logoutErr
}

// ListSessions returns every WhatsApp device that has completed QR
// verification. Sessions that are still mid-pairing (Store.ID == nil) are
// excluded so they don't appear in the registered devices list.
func (m *Manager) ListSessions() []SessionInfo {
	seen := map[string]bool{}
	out := []SessionInfo{}

	m.mu.RLock()
	for _, s := range m.sessions {
		if s == nil || s.Client == nil || s.Client.Store.ID == nil {
			continue
		}
		numID := s.Client.Store.ID.User
		if numID == "" {
			s.mu.RLock()
			numID = s.NumberID
			s.mu.RUnlock()
		}
		if numID == "" || seen[numID] {
			continue
		}
		seen[numID] = true
		out = append(out, SessionInfo{
			NumberID:   numID,
			Connected:  s.Client.IsConnected(),
			HasFile:    true,
			Paired:     true,
			PairingKey: s.PairingKey,
		})
	}
	m.mu.RUnlock()

	return out
}

func (m *Manager) RecoverConnected() {
	if _, err := os.Stat(m.SessionDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(m.SessionDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".db") {
			fileNameID := strings.TrimSuffix(entry.Name(), ".db")

			// A corrupt or half-written session file must not crash the daemon
			// on boot. Recover per file and skip the bad ones.
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[SessionManager] skipping corrupt session %s: %v", fileNameID, r)
					}
				}()

				client, container, err := m.initClient(fileNameID)
				if err != nil || client.Store.ID == nil {
					if container != nil {
						container.Close()
					}
					// Remove orphaned unpaired .db files (never completed QR verification).
					dbPath := filepath.Join(m.SessionDir, fileNameID+".db")
					os.Remove(dbPath)
					return
				}

				realNumber := client.Store.ID.User
				if realNumber == "" {
					realNumber = fileNameID
				}

				dbPath := filepath.Join(m.SessionDir, fileNameID+".db")
				// If file had a temp name like qr-..., rename it to real phone number
				if fileNameID != realNumber {
					newDbPath := filepath.Join(m.SessionDir, realNumber+".db")
					container.Close()
					if err := os.Rename(dbPath, newDbPath); err == nil {
						dbPath = newDbPath
						client, container, err = m.initClient(realNumber)
						if err != nil {
							return
						}
					} else {
						// Reopen container if rename failed
						client, container, _ = m.initClient(fileNameID)
					}
				}

				m.mu.Lock()
				if _, exists := m.sessions[realNumber]; exists {
					m.mu.Unlock()
					if container != nil {
						container.Close()
					}
					return
				}
				session := &Session{Client: client, Container: container, DBPath: dbPath, NumberID: realNumber, PairingKey: m.loadPairingKey(realNumber)}
				m.sessions[realNumber] = session
				m.mu.Unlock()

				nid := realNumber // capture for goroutine
				client.AddEventHandler(func(evt interface{}) {
					m.handleEvent(nid, evt)
				})

				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[SessionManager] connect panic for %s: %v", nid, r)
						}
					}()
					client.Connect()
				}()
			}()
		}
	}
}

// StopAll disconnects all active sessions gracefully.
func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		m.Stop(id, true)
	}
}

// StartQRPairing starts a new pairing session with an auto-generated ID.
// Phone number is not required; it is detected from WhatsApp after QR scanning.
func (m *Manager) StartQRPairing() (string, error) {
	tempID := fmt.Sprintf("qr-%d", time.Now().UnixMilli())
	if err := m.StartPairing(tempID); err != nil {
		return "", err
	}
	return tempID, nil
}

// Pair is an alias for StartPairing, used by the HTTP handler layer.
func (m *Manager) Pair(numberID string) error {
	return m.StartPairing(numberID)
}

// SendMessage is an alias for SendText, used by the HTTP handler layer.
func (m *Manager) SendMessage(numberID, to, text string) error {
	return m.SendText(numberID, to, text)
}

// SendGroupMessage is an alias for SendGroupText, used by the HTTP handler layer.
func (m *Manager) SendGroupMessage(numberID, groupJID, text string) error {
	return m.SendGroupText(numberID, groupJID, text)
}

// GetGroups is an alias for ListGroups, used by the HTTP handler layer.
func (m *Manager) GetGroups(numberID string) ([]GroupInfo, error) {
	return m.ListGroups(numberID)
}

// GetContacts is an alias for ListContacts, used by the HTTP handler layer.
func (m *Manager) GetContacts(numberID string) ([]ContactInfo, error) {
	return m.ListContacts(numberID)
}

// Logout disconnects a session and optionally keeps the session data.
func (m *Manager) Logout(numberID string, keepSession bool) error {
	return m.Stop(numberID, keepSession)
}

// FetchAllConnectedData pulls contacts and groups from every currently connected session.
func (m *Manager) FetchAllConnectedData() (*AllSyncResult, error) {
	sessions := m.ListSessions()
	result := &AllSyncResult{
		Results: []SessionSyncData{},
	}

	// If no connected sessions found via ListSessions, check all loaded sessions
	if len(sessions) == 0 {
		m.mu.RLock()
		for id := range m.sessions {
			sessions = append(sessions, SessionInfo{NumberID: id, Connected: true})
		}
		m.mu.RUnlock()
	}

	for _, s := range sessions {
		syncData := SessionSyncData{
			NumberID: s.NumberID,
			Phone:    s.NumberID,
			Contacts: []ContactInfo{},
			Groups:   []GroupInfo{},
		}

		contacts, err := m.ListContacts(s.NumberID)
		if err != nil {
			syncData.Error = fmt.Sprintf("contacts err: %v", err)
		} else {
			syncData.Contacts = contacts
			result.TotalContacts += len(contacts)
		}

		groups, err := m.ListGroups(s.NumberID)
		if err != nil {
			if syncData.Error != "" {
				syncData.Error += "; "
			}
			syncData.Error += fmt.Sprintf("groups err: %v", err)
		} else {
			syncData.Groups = groups
			result.TotalGroups += len(groups)
		}

		result.Results = append(result.Results, syncData)
		result.TotalSessions++
	}

	return result, nil
}

// FetchSessionData pulls contacts and groups for a specific session or all sessions.
func (m *Manager) FetchSessionData(numberID string) (*SessionSyncData, error) {
	if numberID == "" || numberID == "all" || strings.EqualFold(numberID, "semua") {
		allData, err := m.FetchAllConnectedData()
		if err != nil {
			return nil, err
		}
		var combinedContacts []ContactInfo
		var combinedGroups []GroupInfo
		seenC := map[string]bool{}
		seenG := map[string]bool{}
		for _, res := range allData.Results {
			for _, c := range res.Contacts {
				k := c.Phone
				if k == "" {
					k = c.JID
				}
				if !seenC[k] {
					seenC[k] = true
					combinedContacts = append(combinedContacts, c)
				}
			}
			for _, g := range res.Groups {
				if !seenG[g.JID] {
					seenG[g.JID] = true
					combinedGroups = append(combinedGroups, g)
				}
			}
		}
		return &SessionSyncData{
			NumberID: "all",
			Phone:    "all",
			Contacts: combinedContacts,
			Groups:   combinedGroups,
		}, nil
	}

	session := m.findSession(numberID)
	if session == nil {
		return nil, fmt.Errorf("nomor %s tidak ditemukan atau tidak terhubung", numberID)
	}

	numID := session.NumberID
	if numID == "" && session.Client != nil && session.Client.Store != nil && session.Client.Store.ID != nil {
		numID = session.Client.Store.ID.User
	}
	if numID == "" {
		numID = numberID
	}

	syncData := &SessionSyncData{
		NumberID: numID,
		Phone:    numID,
		Contacts: []ContactInfo{},
		Groups:   []GroupInfo{},
	}

	contacts, err := m.ListContacts(numID)
	if err != nil {
		syncData.Error = fmt.Sprintf("contacts err: %v", err)
	} else {
		syncData.Contacts = contacts
	}

	groups, err := m.ListGroups(numID)
	if err != nil {
		if syncData.Error != "" {
			syncData.Error += "; "
		}
		syncData.Error += fmt.Sprintf("groups err: %v", err)
	} else {
		syncData.Groups = groups
	}

	return syncData, nil
}

// GetAllContacts aggregates contacts from all sessions, deduplicated by phone/JID.
// It directly inspects local SQLite stores so contacts are available even during reconnection.
func (m *Manager) GetAllContacts() ([]ContactInfo, error) {
	seen := make(map[string]bool)
	var allContacts []ContactInfo

	m.mu.RLock()
	var sessionList []*Session
	for _, s := range m.sessions {
		if s != nil && s.Client != nil && s.Client.Store != nil && s.Client.Store.Contacts != nil {
			sessionList = append(sessionList, s)
		}
	}
	m.mu.RUnlock()

	for _, s := range sessionList {
		contacts, err := s.Client.Store.Contacts.GetAllContacts(context.Background())
		if err != nil {
			continue
		}
		for jid, c := range contacts {
			if jid.Server != types.DefaultUserServer {
				continue
			}
			phone := strings.Split(jid.User, ":")[0]
			if phone == "" {
				continue
			}
			name := c.FullName
			if name == "" {
				name = c.PushName
			}
			if name == "" {
				name = c.BusinessName
			}
			if name == "" {
				name = c.FirstName
			}
			if name == "" {
				name = "Kontak " + phone
			}
			if !seen[phone] {
				seen[phone] = true
				allContacts = append(allContacts, ContactInfo{
					JID:   jid.String(),
					Phone: phone,
					Name:  name,
				})
			}
		}
	}
	return allContacts, nil
}

// GetAllGroups aggregates groups from all connected sessions, deduplicated by JID.
func (m *Manager) GetAllGroups() ([]GroupInfo, error) {
	seen := make(map[string]bool)
	var allGroups []GroupInfo

	m.mu.RLock()
	var sessionList []*Session
	for _, s := range m.sessions {
		if s != nil && s.Client != nil && s.Client.IsConnected() {
			sessionList = append(sessionList, s)
		}
	}
	m.mu.RUnlock()

	for _, s := range sessionList {
		groups, err := s.Client.GetJoinedGroups(context.Background())
		if err != nil {
			continue
		}
		for _, g := range groups {
			if !seen[g.JID.String()] {
				seen[g.JID.String()] = true
				allGroups = append(allGroups, GroupInfo{
					JID:              g.JID.String(),
					Name:             g.Name,
					Topic:            g.Topic,
					ParticipantCount: len(g.Participants),
				})
			}
		}
	}
	return allGroups, nil
}



