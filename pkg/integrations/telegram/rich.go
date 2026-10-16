package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (c *Client) SendRichHTML(ctx context.Context, html string, replyTo int) (int, error) {
	if strings.TrimSpace(html) == "" {
		return 0, fmt.Errorf("telegram rich message is empty")
	}

	params := &bot.SendRichMessageParams{
		ChatID:          c.chatID,
		MessageThreadID: c.threadID,
		RichMessage:     models.InputRichMessage{HTML: html},
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

func (c *Client) EditRichHTML(ctx context.Context, messageID int, html string) error {
	if messageID <= 0 {
		return fmt.Errorf("telegram message id is required")
	}
	if strings.TrimSpace(html) == "" {
		return fmt.Errorf("telegram rich message is empty")
	}

	_, err := c.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      c.chatID,
		MessageID:   messageID,
		RichMessage: &models.InputRichMessage{HTML: html},
	})
	if err != nil {
		return fmt.Errorf("failed to edit telegram rich message %d in %s: %w", messageID, c.chatID, err)
	}
	return nil
}
