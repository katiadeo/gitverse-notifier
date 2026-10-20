package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const telegramAPIBase = "https://api.telegram.org"

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func (c *Client) SendRich(ctx context.Context, richHTML string, replyTo int) (int, error) {
	if strings.TrimSpace(richHTML) == "" {
		return 0, fmt.Errorf("telegram rich message is empty")
	}

	params := &bot.SendRichMessageParams{
		ChatID:          c.chatID,
		MessageThreadID: c.threadID,
		RichMessage:     models.InputRichMessage{HTML: richHTML},
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{
			MessageID:                replyTo,
			AllowSendingWithoutReply: true,
		}
	}

	msg, err := c.bot.SendRichMessage(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("failed to send telegram rich message to %s: %w", c.chatID, err)
	}
	if msg == nil {
		return 0, nil
	}
	return msg.ID, nil
}

func (c *Client) EditRich(ctx context.Context, messageID int, richHTML string) error {
	if messageID <= 0 {
		return fmt.Errorf("telegram message id is required")
	}
	if strings.TrimSpace(richHTML) == "" {
		return fmt.Errorf("telegram rich message is empty")
	}

	payload := map[string]any{
		"chat_id":    c.chatID,
		"message_id": messageID,
		"rich_message": models.InputRichMessage{
			HTML: richHTML,
		},
	}
	if err := c.callAPI(ctx, "editMessageText", payload); err != nil {
		return fmt.Errorf("failed to edit telegram rich message %d in %s: %w", messageID, c.chatID, err)
	}
	return nil
}

func (c *Client) callAPI(ctx context.Context, method string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode telegram %s payload: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read telegram %s response: %w", method, err)
	}

	var parsed telegramAPIResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("telegram %s returned %d: %s", method, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if !parsed.OK {
		desc := strings.TrimSpace(parsed.Description)
		if desc == "" {
			desc = strings.TrimSpace(string(raw))
		}
		return fmt.Errorf("%s", desc)
	}
	return nil
}

func (c *Client) methodURL(method string) string {
	base := strings.TrimRight(c.apiBase, "/")
	if base == "" {
		base = telegramAPIBase
	}
	return fmt.Sprintf("%s/bot%s/%s", base, c.token, method)
}
