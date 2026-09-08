package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookSend(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		got <- string(buf)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wh := NewWebhook(srv.URL)
	if err := wh.Send(context.Background(), Message{Title: "t", Body: "b", Level: "warning"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case body := <-got:
		if body == "" {
			t.Fatal("empty webhook body")
		}
	default:
		t.Fatal("webhook not called")
	}
}

func TestNotifierNames(t *testing.T) {
	cases := map[string]Notifier{
		"telegram": NewTelegram("tok", "chat"),
		"dingtalk": NewDingTalk("https://example.com", ""),
		"feishu":   NewFeishu("https://example.com"),
		"email":    NewEmail("smtp:587", "u", "p", "f@x.com", []string{"t@x.com"}),
		"webhook":  NewWebhook("https://example.com"),
	}
	for want, n := range cases {
		if n.Name() != want {
			t.Errorf("want name %s, got %s", want, n.Name())
		}
	}
}

func TestTelegramMissingConfig(t *testing.T) {
	tg := NewTelegram("", "")
	if err := tg.Send(context.Background(), Message{Title: "x"}); err == nil {
		t.Fatal("expected error for missing config")
	}
}
