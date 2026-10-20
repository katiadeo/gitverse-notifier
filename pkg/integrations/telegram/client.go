package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitverse-notifier/pkg/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Client struct {
	bot       *bot.Bot
	http      *http.Client
	token     string
	apiBase   string
	chatID    string
	threadID  int
	parseMode models.ParseMode
}

func NewTelegramClient() (*Client, error) {
	cfg := config.GlobalConfig
	token := strings.TrimSpace(cfg.TelegramBotToken)
	chatID := strings.TrimSpace(cfg.TelegramChatID)

	if token == "" || chatID == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID configs required")
	}

	httpClient, opts, err := botOptions(cfg.TelegramProxyURL)
	if err != nil {
		return nil, err
	}

	tgBot, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to setup telegram integration: %w", err)
	}

	return &Client{
		bot:      tgBot,
		http:     httpClient,
		token:    token,
		chatID:   chatID,
		threadID: cfg.TelegramThreadId,
		// TODO: Неправильный тип вернет ошибки при отправке, но я чет не хочу писать отдельную валидацию, будет на совести пользователя
		parseMode: models.ParseMode(cfg.TelegramParseMode),
	}, nil
}

func botOptions(proxyURL string) (*http.Client, []bot.Option, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return httpClient, []bot.Option{bot.WithHTTPClient(10*time.Second, httpClient)}, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid TELEGRAM_PROXY_URL %s: %w", proxyURL, err)
	}

	httpClient.Transport = &http.Transport{
		Proxy: http.ProxyURL(parsed),
	}

	return httpClient, []bot.Option{
		bot.WithHTTPClient(10*time.Second, httpClient),
	}, nil
}

func (c *Client) SendMessage(ctx context.Context, text string, replyTo int) (int, error) {
	disabled := true
	params := &bot.SendMessageParams{
		ChatID:          c.chatID,
		MessageThreadID: c.threadID,
		Text:            text,
		ParseMode:       c.parseMode,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &disabled,
		},
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{
			MessageID:                replyTo,
			AllowSendingWithoutReply: true,
		}
	}

	msg, err := c.bot.SendMessage(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("failed to send telegram message to %s: %w", c.chatID, err)
	}
	if msg == nil {
		return 0, nil
	}
	return msg.ID, nil
}

func (c *Client) EditMessage(ctx context.Context, messageID int, text string) error {
	if messageID <= 0 {
		return fmt.Errorf("telegram message id is required")
	}
	disabled := true
	_, err := c.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    c.chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: c.parseMode,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &disabled,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to edit telegram message %d in %s: %w", messageID, c.chatID, err)
	}
	return nil
}

func (c *Client) DeleteMessages(ctx context.Context, messageIDs []int) error {
	ids := uniquePositiveIDs(messageIDs)
	if len(ids) == 0 {
		return nil
	}
	if len(ids) == 1 {
		_, err := c.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    c.chatID,
			MessageID: ids[0],
		})
		if err != nil {
			return fmt.Errorf("failed to delete telegram message %d in %s: %w", ids[0], c.chatID, err)
		}
		return nil
	}
	_, err := c.bot.DeleteMessages(ctx, &bot.DeleteMessagesParams{
		ChatID:     c.chatID,
		MessageIDs: ids,
	})
	if err != nil {
		return fmt.Errorf("failed to delete telegram messages in %s: %w", c.chatID, err)
	}
	return nil
}

func uniquePositiveIDs(ids []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func IsNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "message is not modified")
}

func IsMessageGone(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "message to edit not found") ||
		strings.Contains(msg, "message to delete not found") ||
		strings.Contains(msg, "message can't be edited")
}
