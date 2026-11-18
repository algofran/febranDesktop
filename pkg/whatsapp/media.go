package whatsapp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// recipientJID turns a phone number or group id into a JID (bare digits need
// their domain attached: types.ParseJID would treat them as a server name).
func recipientJID(to string, group bool) (types.JID, error) {
	to = strings.TrimSpace(to)
	if !strings.Contains(to, "@") {
		if group {
			to += "@g.us"
		} else {
			to = strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, to) + "@s.whatsapp.net"
		}
	}
	jid, err := types.ParseJID(to)
	if err != nil {
		return jid, fmt.Errorf("invalid recipient JID (%s): %w", to, err)
	}
	return jid, nil
}

// maxInboundMedia caps attachments forwarded to the backend (sent base64 in JSON, so a 16 MB file
// is ~22 MB on the wire, under the backend's 52 MB body limit).
const maxInboundMedia = 16 << 20

// mediaMessage is what whatsmeow needs to download an attachment and how big it is.
type mediaMessage interface {
	whatsmeow.DownloadableMessage
	GetFileLength() uint64
}

// inboundMedia picks the attachment of a received message, if any, with its kind, document file
// name and caption. Stickers and other kinds are ignored.
func inboundMedia(msg *waE2E.Message) (mediaMessage, string, string, string) {
	switch {
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage(), "image", "", msg.GetImageMessage().GetCaption()
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage(), "video", "", msg.GetVideoMessage().GetCaption()
	case msg.GetAudioMessage() != nil:
		return msg.GetAudioMessage(), "audio", "", ""
	case msg.GetDocumentMessage() != nil:
		d := msg.GetDocumentMessage()
		return d, "document", d.GetFileName(), d.GetCaption()
	case msg.GetDocumentWithCaptionMessage().GetMessage().GetDocumentMessage() != nil:
		d := msg.GetDocumentWithCaptionMessage().GetMessage().GetDocumentMessage()
		return d, "document", d.GetFileName(), d.GetCaption()
	}
	return nil, "", "", ""
}

// MediaRetention is how long received attachments stay on this computer (and downloadable).
const MediaRetention = 7 * 24 * time.Hour

func (m *Manager) mediaDir() string { return filepath.Join(m.SessionDir, "media") }

// mediaFile is where an attachment is kept; both parts are reduced to letters and digits so a
// reference coming back from the backend can never point outside the media folder.
func (m *Manager) mediaFile(numberID, ref string) (string, error) {
	keep := func(s string) string {
		return strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				return r
			}
			return -1
		}, s)
	}
	n, r := keep(numberID), keep(ref)
	if n == "" || r == "" {
		return "", fmt.Errorf("invalid media reference")
	}
	return filepath.Join(m.mediaDir(), n+"_"+r), nil
}

// SaveInboundMedia keeps a received attachment and returns the reference the backend uses to ask for it.
func (m *Manager) SaveInboundMedia(numberID, messageID string, data []byte) (string, error) {
	p, err := m.mediaFile(numberID, messageID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(m.mediaDir(), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return "", err
	}
	base := filepath.Base(p) // "<number>_<ref>"
	return base[strings.IndexByte(base, '_')+1:], nil
}

// InboundMedia returns a kept attachment; gone once older than MediaRetention.
func (m *Manager) InboundMedia(numberID, ref string) ([]byte, error) {
	p, err := m.mediaFile(numberID, ref)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(p); err != nil || time.Since(st.ModTime()) > MediaRetention {
		return nil, fmt.Errorf("media no longer available")
	}
	return os.ReadFile(p)
}

// CleanupMedia deletes attachments older than MediaRetention.
func (m *Manager) CleanupMedia() {
	entries, _ := os.ReadDir(m.mediaDir())
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > MediaRetention {
			_ = os.Remove(filepath.Join(m.mediaDir(), e.Name()))
		}
	}
}

// SendMedia uploads data to WhatsApp and sends it as an image, video or
// document (chosen from the sniffed content type). caption may be empty.
func (m *Manager) SendMedia(numberID, to string, group bool, data []byte, fileName, caption string) error {
	session := m.findSession(numberID)
	if session == nil {
		return fmt.Errorf("session not found")
	}
	jid, err := recipientJID(to, group)
	if err != nil {
		return err
	}
	mime := http.DetectContentType(data)
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]
	}

	mediaType := whatsmeow.MediaDocument
	switch {
	case strings.HasPrefix(mime, "image/"):
		mediaType = whatsmeow.MediaImage
	case strings.HasPrefix(mime, "video/"):
		mediaType = whatsmeow.MediaVideo
	}

	ctx := context.Background()
	up, err := session.Client.Upload(ctx, data, mediaType)
	if err != nil {
		return fmt.Errorf("upload media: %w", err)
	}

	var msg *waE2E.Message
	switch mediaType {
	case whatsmeow.MediaImage:
		msg = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String(caption), Mimetype: proto.String(mime),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength),
		}}
	case whatsmeow.MediaVideo:
		msg = &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			Caption: proto.String(caption), Mimetype: proto.String(mime),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength),
		}}
	default:
		if fileName == "" {
			fileName = "dokumen"
		}
		msg = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
			Caption: proto.String(caption), Mimetype: proto.String(mime),
			FileName: proto.String(fileName), Title: proto.String(fileName),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength),
		}}
	}
	_, err = session.Client.SendMessage(ctx, jid, msg)
	return err
}
