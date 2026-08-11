package reload

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHubNotifyReloadsSSEClient(t *testing.T) {
	t.Parallel()

	hub, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	eventsURL := "http://" + hub.addr + "/events"
	resp, err := http.Get(eventsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events status = %d", resp.StatusCode)
	}

	got := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "event:") {
				got <- strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				return
			}
		}
	}()

	// Allow the client to register before notify.
	time.Sleep(20 * time.Millisecond)

	notifyResp, err := http.Post(hub.NotifyURL(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, notifyResp.Body)
	_ = notifyResp.Body.Close()
	if notifyResp.StatusCode != http.StatusNoContent {
		t.Fatalf("notify status = %d", notifyResp.StatusCode)
	}

	select {
	case ev := <-got:
		if ev != "reload" {
			t.Fatalf("event = %q, want reload", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reload event")
	}

	scriptResp, err := http.Get(hub.ScriptURL())
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(scriptResp.Body)
	_ = scriptResp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "EventSource") {
		t.Fatalf("script missing EventSource: %s", body)
	}
}
