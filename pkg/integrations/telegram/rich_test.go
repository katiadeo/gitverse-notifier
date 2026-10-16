package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	b, err := bot.New("tok",
		bot.WithSkipGetMe(),
		bot.WithServerURL(srv.URL),
		bot.WithHTTPClient(time.Second, srv.Client()),
	)
	require.NoError(t, err)
	return &Client{bot: b, chatID: "-1001"}
}

func TestSendRichHTML(t *testing.T) {
	t.Parallel()

	var gotMethod, richHTML, replyTo string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.URL.Path
		require.NoError(t, r.ParseMultipartForm(1<<20))
		richHTML = r.FormValue("rich_message")
		replyTo = r.FormValue("reply_parameters")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":77}}`))
	})

	id, err := c.SendRichHTML(context.Background(), `<details><summary>Комментарии · 1</summary><p>↳ bob</p></details>`, 99)
	require.NoError(t, err)
	require.Equal(t, 77, id)
	require.Equal(t, "/bottok/sendRichMessage", gotMethod)
	require.Equal(t, `<details><summary>Комментарии · 1</summary><p>↳ bob</p></details>`, decodeRichHTML(t, richHTML))
	require.Contains(t, replyTo, `"message_id":99`)
}

func TestEditRichHTML(t *testing.T) {
	t.Parallel()

	var gotMethod, richHTML string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.URL.Path
		require.NoError(t, r.ParseMultipartForm(1<<20))
		richHTML = r.FormValue("rich_message")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":15}}`))
	})

	require.NoError(t, c.EditRichHTML(context.Background(), 15, `<details><summary>x</summary></details>`))
	require.Equal(t, "/bottok/editMessageText", gotMethod)
	require.Equal(t, `<details><summary>x</summary></details>`, decodeRichHTML(t, richHTML))
}

func TestSendRichHTML_APIError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: message is not modified"}`))
	})
	err := c.EditRichHTML(context.Background(), 15, `<p>same</p>`)
	require.Error(t, err)
	require.True(t, IsNotModified(err))
}

func decodeRichHTML(t *testing.T, raw string) string {
	t.Helper()
	var msg models.InputRichMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &msg))
	return msg.HTML
}
