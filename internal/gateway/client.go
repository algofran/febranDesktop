package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	pathpkg "path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/algofran/febranDesktop/pkg/whatsapp"
	"github.com/coder/websocket"
)

// Message is the wire protocol shared with the central backend.
type Message struct {
	Type      string   `json:"type"`
	ReqID     string   `json:"req_id,omitempty"`
	ServerKey string   `json:"server_key,omitempty"`
	NumberID  string   `json:"number_id,omitempty"`
	Phone     string   `json:"phone,omitempty"`
	To        string   `json:"to,omitempty"`
	Text      string   `json:"text,omitempty"`
	// MediaURL is a backend-relative upload path; Text becomes the caption.
	MediaURL  string   `json:"media_url,omitempty"`
	GroupJID  string   `json:"group_jid,omitempty"`
	Connected bool     `json:"connected,omitempty"`
	DeviceJID string   `json:"device_jid,omitempty"`
	PairingKey string  `json:"pairing_key,omitempty"`
	QR        string   `json:"qr,omitempty"`
	Code      string   `json:"code,omitempty"`
	OK        bool     `json:"ok,omitempty"`
	Error     string   `json:"error,omitempty"`
	Devices   []Device `json:"devices,omitempty"`
	Groups    []Group   `json:"groups,omitempty"`
	Members   []string  `json:"members,omitempty"`
	Contacts  []Contact `json:"contacts,omitempty"`
}

type Device struct {
	NumberID   string `json:"number_id"`
	Connected  bool   `json:"connected"`
	DeviceJID  string `json:"device_jid,omitempty"`
	PairingKey string `json:"pairing_key,omitempty"`
}

type Group struct {
	JID         string `json:"jid"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
}

type Contact struct {
	JID   string `json:"jid"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
}

// Client dials the central backend, registers this node, and executes the
// commands the backend routes to it (pair QR/code, send, groups).
type Client struct {
	url       string
	serverKey string
	nodeID    string
	key       string
	mgr       *whatsapp.Manager

	conn      *websocket.Conn
	writeMu   sync.Mutex
	connected atomic.Bool
}

// closeCodeKeyTaken mirrors the backend's 4000 reject reason sent when a
// server_key is already held by a different node.
const closeCodeKeyTaken = 4000

func NewClient(wsURL, serverKey, key, nodeID string, mgr *whatsapp.Manager) *Client {
	return &Client{url: wsURL, serverKey: serverKey, nodeID: nodeID, key: key, mgr: mgr}
}

func (c *Client) IsConnected() bool { return c.connected.Load() }

// Run dials the backend and reconnects with backoff until ctx is cancelled.
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.connect(ctx)
		c.connected.Store(false)
		if err != nil {
			log.Printf("[ws] backend connect failed: %v (retry in %s)", err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) connect(ctx context.Context) error {
	u := c.url + "?server_key=" + url.QueryEscape(c.serverKey) + "&node_id=" + url.QueryEscape(c.nodeID) + "&key=" + url.QueryEscape(c.key)
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	c.conn = conn
	c.writeMu.Unlock()
	c.connected.Store(true)
	log.Printf("[ws] connected to backend as %q", c.serverKey)

	c.sendHello()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			c.writeMu.Lock()
			c.conn = nil
			c.writeMu.Unlock()
			if websocket.CloseStatus(err) == closeCodeKeyTaken {
				log.Printf("❌ server_key %q already in use by ANOTHER node — pairing would hit the wrong server. Change GATEWAY_SERVER_KEY (Settings → Central Backend Connection) to a unique key.", c.serverKey)
			} else {
				log.Printf("[ws] disconnected: %v", err)
			}
			return err
		}
		var m Message
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		go c.handle(m)
	}
}

func (c *Client) sendHello() {
	var devices []Device
	for _, s := range c.mgr.ListSessions() {
		devices = append(devices, Device{
			NumberID:   s.NumberID,
			Connected:  s.Connected,
			DeviceJID:  c.mgr.DeviceJID(s.NumberID),
			PairingKey: s.PairingKey,
		})
	}
	c.write(Message{Type: "hello", ServerKey: c.serverKey, Devices: devices})
}

// PushDeviceUpdate reports a single device's pairing state to the backend.
func (c *Client) PushDeviceUpdate(numberID string, connected bool, deviceJID, pairingKey string) {
	c.write(Message{Type: "device_update", NumberID: numberID, Connected: connected, DeviceJID: deviceJID, PairingKey: pairingKey})
}

func (c *Client) write(m Message) {
	if !c.connected.Load() {
		return
	}
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.conn == nil {
		return
	}
	_ = c.conn.Write(ctx, websocket.MessageText, b)
}

func (c *Client) handle(m Message) {
	switch m.Type {
	case "pair_qr":
		c.pairQR(m)
	case "pair_code":
		c.pairCode(m)
	case "send":
		c.doSend(m)
	case "send_group":
		c.doSendGroup(m)
	case "list_groups":
		c.listGroups(m)
	case "list_contacts":
		c.listContacts(m)
	case "sync_all":
		c.syncAll(m)
	case "group_members_req":
		c.groupMembers(m)
	case "disconnect":
		c.doDisconnect(m)
	}
}

func (c *Client) pairQR(m Message) {
	_ = c.mgr.StartPairing(m.NumberID) // ignore "already pairing"
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if qr, err := c.mgr.GetQR(m.NumberID); err == nil && qr != "" {
			c.write(Message{Type: "qr", ReqID: m.ReqID, NumberID: m.NumberID, QR: qr})
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
	c.write(Message{Type: "qr", ReqID: m.ReqID, NumberID: m.NumberID, Error: "QR not available"})
}

func (c *Client) pairCode(m Message) {
	code, err := c.mgr.PairWithCode(m.NumberID, m.Phone)
	if err != nil {
		c.write(Message{Type: "pairing_code", ReqID: m.ReqID, NumberID: m.NumberID, Error: err.Error()})
		return
	}
	c.write(Message{Type: "pairing_code", ReqID: m.ReqID, NumberID: m.NumberID, Code: code})
}

// mediaPathPrefix is the only place a node downloads attachments from, on the
// backend it is connected to — never an arbitrary URL from the message.
const mediaPathPrefix = "/api/v1/uploads/media/"

const maxMediaBytes = 50 << 20

var mediaHTTP = &http.Client{Timeout: 60 * time.Second}

// fetchMedia downloads an attachment from the connected backend.
func (c *Client) fetchMedia(path string) ([]byte, string, error) {
	if !strings.HasPrefix(path, mediaPathPrefix) || strings.Contains(path, "..") || strings.ContainsAny(path, "?#\\") {
		return nil, "", fmt.Errorf("media path not allowed")
	}
	base, err := url.Parse(c.url)
	if err != nil {
		return nil, "", err
	}
	switch base.Scheme {
	case "wss":
		base.Scheme = "https"
	case "ws":
		base.Scheme = "http"
	}
	base.Path, base.RawQuery = path, ""
	resp, err := mediaHTTP.Get(base.String())
	if err != nil {
		return nil, "", fmt.Errorf("download media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download media: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("download media: %w", err)
	}
	if len(data) > maxMediaBytes {
		return nil, "", fmt.Errorf("media too large")
	}
	return data, pathpkg.Base(path), nil
}

// sendMedia downloads then sends an attachment to a contact or group.
func (c *Client) sendMedia(m Message, to string, group bool) error {
	data, name, err := c.fetchMedia(m.MediaURL)
	if err != nil {
		return err
	}
	return c.mgr.SendMedia(m.NumberID, to, group, data, name, m.Text)
}

func (c *Client) doSend(m Message) {
	var err error
	if m.MediaURL != "" {
		err = c.sendMedia(m, m.To, false)
	} else {
		err = c.mgr.SendMessage(m.NumberID, m.To, m.Text)
	}
	if err != nil {
		c.write(Message{Type: "send_result", ReqID: m.ReqID, NumberID: m.NumberID, OK: false, Error: err.Error()})
		return
	}
	c.write(Message{Type: "send_result", ReqID: m.ReqID, NumberID: m.NumberID, OK: true})
}

func (c *Client) doSendGroup(m Message) {
	var err error
	if m.MediaURL != "" {
		err = c.sendMedia(m, m.GroupJID, true)
	} else {
		err = c.mgr.SendGroupMessage(m.NumberID, m.GroupJID, m.Text)
	}
	if err != nil {
		c.write(Message{Type: "send_result", ReqID: m.ReqID, NumberID: m.NumberID, OK: false, Error: err.Error()})
		return
	}
	c.write(Message{Type: "send_result", ReqID: m.ReqID, NumberID: m.NumberID, OK: true})
}

func (c *Client) listGroups(m Message) {
	groups, err := c.mgr.ListGroups(m.NumberID)
	if err != nil {
		c.write(Message{Type: "groups", ReqID: m.ReqID, NumberID: m.NumberID, Error: err.Error()})
		return
	}
	gs := make([]Group, 0, len(groups))
	for _, g := range groups {
		gs = append(gs, Group{JID: g.JID, Name: g.Name, MemberCount: g.ParticipantCount})
	}
	c.write(Message{Type: "groups", ReqID: m.ReqID, NumberID: m.NumberID, Groups: gs})
}

func (c *Client) listContacts(m Message) {
	contacts, err := c.mgr.ListContacts(m.NumberID)
	if err != nil {
		c.write(Message{Type: "contacts", ReqID: m.ReqID, NumberID: m.NumberID, Error: err.Error()})
		return
	}
	cs := make([]Contact, 0, len(contacts))
	for _, ct := range contacts {
		cs = append(cs, Contact{JID: ct.JID, Phone: ct.Phone, Name: ct.Name})
	}
	c.write(Message{Type: "contacts", ReqID: m.ReqID, NumberID: m.NumberID, Contacts: cs})
}

func (c *Client) groupMembers(m Message) {
	members, err := c.mgr.GetGroupMembers(m.NumberID, m.GroupJID)
	if err != nil {
		c.write(Message{Type: "group_members", ReqID: m.ReqID, NumberID: m.NumberID, Error: err.Error()})
		return
	}
	c.write(Message{Type: "group_members", ReqID: m.ReqID, NumberID: m.NumberID, Members: members})
}

func (c *Client) doDisconnect(m Message) {
	err := c.mgr.Stop(m.NumberID, false)
	if err != nil {
		c.write(Message{Type: "disconnect", ReqID: m.ReqID, NumberID: m.NumberID, Error: err.Error()})
		return
	}
	c.write(Message{Type: "disconnect", ReqID: m.ReqID, NumberID: m.NumberID, OK: true})
}

// PushSyncData sends contacts and groups of a session to the central backend.
func (c *Client) PushSyncData(numberID string, contacts []Contact, groups []Group) {
	c.write(Message{
		Type:     "sync_data",
		NumberID: numberID,
		Contacts: contacts,
		Groups:   groups,
	})
}

func (c *Client) syncAll(m Message) {
	allData, err := c.mgr.FetchAllConnectedData()
	if err != nil {
		c.write(Message{Type: "sync_all_result", ReqID: m.ReqID, OK: false, Error: err.Error()})
		return
	}
	for _, res := range allData.Results {
		var cs []Contact
		for _, ct := range res.Contacts {
			cs = append(cs, Contact{JID: ct.JID, Phone: ct.Phone, Name: ct.Name})
		}
		var gs []Group
		for _, gp := range res.Groups {
			gs = append(gs, Group{JID: gp.JID, Name: gp.Name, MemberCount: gp.ParticipantCount})
		}
		c.PushSyncData(res.NumberID, cs, gs)
	}
	c.write(Message{Type: "sync_all_result", ReqID: m.ReqID, OK: true})
}

// SyncAllToBackend pulls all contacts & groups from connected sessions and pushes them to backend.
func (c *Client) SyncAllToBackend() (*whatsapp.AllSyncResult, error) {
	allData, err := c.mgr.FetchAllConnectedData()
	if err != nil {
		return nil, err
	}
	for _, res := range allData.Results {
		var cs []Contact
		for _, ct := range res.Contacts {
			cs = append(cs, Contact{JID: ct.JID, Phone: ct.Phone, Name: ct.Name})
		}
		var gs []Group
		for _, gp := range res.Groups {
			gs = append(gs, Group{JID: gp.JID, Name: gp.Name, MemberCount: gp.ParticipantCount})
		}
		c.PushSyncData(res.NumberID, cs, gs)
	}
	return allData, nil
}

// SyncSessionToBackend pulls contacts & groups for a single session and pushes them to backend.
func (c *Client) SyncSessionToBackend(numberID string) (*whatsapp.SessionSyncData, error) {
	data, err := c.mgr.FetchSessionData(numberID)
	if err != nil {
		return nil, err
	}
	var cs []Contact
	for _, ct := range data.Contacts {
		cs = append(cs, Contact{JID: ct.JID, Phone: ct.Phone, Name: ct.Name})
	}
	var gs []Group
	for _, gp := range data.Groups {
		gs = append(gs, Group{JID: gp.JID, Name: gp.Name, MemberCount: gp.ParticipantCount})
	}
	c.PushSyncData(numberID, cs, gs)
	return data, nil
}

