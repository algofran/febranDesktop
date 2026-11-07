package whatsapp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
