package client

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/bbernhard/signal-cli-rest-api/utils"
)

// fakeSignalCli is a loopback TCP server that speaks the line-delimited
// JSON-RPC2 protocol JsonRpc2Client uses to talk to signal-cli. It lets the
// tests drive the transport contract over a real net.Conn without a live
// signal-cli daemon. Accepted connections are intentionally left open for the
// duration of the test so the client's ReceiveData reader stays parked on the
// socket instead of busy-looping on EOF; only the listener is closed.
type fakeSignalCli struct {
	ln     net.Listener
	connCh chan net.Conn
}

func newFakeSignalCli(t *testing.T) *fakeSignalCli {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeSignalCli{ln: ln, connCh: make(chan net.Conn, 1)}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		f.connCh <- conn
	}()
	return f
}

func (f *fakeSignalCli) addr() string { return f.ln.Addr().String() }

func (f *fakeSignalCli) accepted(t *testing.T) net.Conn {
	t.Helper()
	select {
	case c := <-f.connCh:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("server did not accept a connection")
		return nil
	}
}

func (f *fakeSignalCli) close() { f.ln.Close() }

func newDialedClient(t *testing.T, f *fakeSignalCli, number string) *JsonRpc2Client {
	t.Helper()
	c := NewJsonRpc2Client(utils.NewSignalCliApiConfig(), number)
	if err := c.Dial(f.addr()); err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

// readRequestLine reads one newline-delimited JSON-RPC request frame from the
// server side of the connection and decodes it.
func readRequestLine(t *testing.T, srv net.Conn) map[string]interface{} {
	t.Helper()
	line, err := bufio.NewReader(srv).ReadString('\n')
	if err != nil {
		t.Errorf("server read request: %v", err)
		return nil
	}
	var req map[string]interface{}
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		t.Errorf("request is not valid JSON: %v (line %q)", err, line)
		return nil
	}
	return req
}

// deliverResponse mirrors what ReceiveData does on the wire - it hands a reply
// to the response channel getRaw registered for the request id. It waits for
// that registration and accesses the map under the same mutex getRaw uses, so
// the exchange is synchronized and free of data races.
func deliverResponse(t *testing.T, c *JsonRpc2Client, id string, resp JsonRpc2MessageResponse) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.receivedResponsesMutex.Lock()
		ch, ok := c.receivedResponsesById[id]
		c.receivedResponsesMutex.Unlock()
		if ok {
			ch <- resp
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("getRaw never registered a response channel for id %q", id)
}

// TestJsonRpc2Client_GetRaw_RequestResponseContract locks the request/response
// transport contract: getRaw serializes a JSON-RPC 2.0 request, writes it as a
// single newline-terminated frame over the real net.Conn (injecting the account
// into params), correlates the reply by id, and returns the raw result payload.
func TestJsonRpc2Client_GetRaw_RequestResponseContract(t *testing.T) {
	f := newFakeSignalCli(t)
	defer f.close()
	number := "+15551230000"
	account := "+15550000000"

	c := newDialedClient(t, f, number)

	type result struct {
		out string
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		out, err := c.getRaw("listAccounts", &account, nil)
		resCh <- result{out, err}
	}()

	srv := f.accepted(t)
	req := readRequestLine(t, srv)
	if req["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want \"2.0\"", req["jsonrpc"])
	}
	if req["method"] != "listAccounts" {
		t.Errorf("method = %v, want \"listAccounts\"", req["method"])
	}
	id, _ := req["id"].(string)
	if id == "" {
		t.Fatal("request id should be a non-empty correlation id")
	}
	if params, ok := req["params"].(map[string]interface{}); !ok || params["account"] != account {
		t.Errorf("params = %v, want account %q injected", req["params"], account)
	}

	deliverResponse(t, c, id, JsonRpc2MessageResponse{
		Id:     id,
		Result: json.RawMessage(`{"accounts":["` + number + `"]}`),
	})

	select {
	case r := <-resCh:
		if r.err != nil {
			t.Fatalf("getRaw returned error: %v", r.err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(r.out), &parsed); err != nil {
			t.Fatalf("result is not valid JSON: %v (got %q)", err, r.out)
		}
		accounts, ok := parsed["accounts"].([]interface{})
		if !ok || len(accounts) != 1 || accounts[0] != number {
			t.Errorf("result accounts = %v, want [%q]", parsed["accounts"], number)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("getRaw did not return; transport or response correlation is broken")
	}
}

// TestJsonRpc2Client_GetRaw_RateLimitError locks the rate-limit error contract:
// a JSON-RPC error with code -5 surfaces as a *RateLimitErrorType carrying the
// parsed challenge tokens.
func TestJsonRpc2Client_GetRaw_RateLimitError(t *testing.T) {
	f := newFakeSignalCli(t)
	defer f.close()
	number := "+15551230000"

	c := newDialedClient(t, f, number)

	errCh := make(chan error, 1)
	go func() {
		_, err := c.getRaw("send", nil, nil)
		errCh <- err
	}()

	srv := f.accepted(t)
	req := readRequestLine(t, srv)
	id, _ := req["id"].(string)

	deliverResponse(t, c, id, JsonRpc2MessageResponse{
		Id: id,
		Err: Error{
			Code:    -5,
			Message: "Rate limit hit",
			Data:    json.RawMessage(`{"response":{"results":[{"token":"tok-1"},{"token":"tok-2"}]}}`),
		},
	})

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a rate-limit error, got nil")
		}
		rlErr, ok := err.(*RateLimitErrorType)
		if !ok {
			t.Fatalf("error type = %T, want *RateLimitErrorType", err)
		}
		if len(rlErr.ChallengeTokens) != 2 || rlErr.ChallengeTokens[0] != "tok-1" || rlErr.ChallengeTokens[1] != "tok-2" {
			t.Errorf("challenge tokens = %v, want [tok-1 tok-2]", rlErr.ChallengeTokens)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("getRaw did not return for the rate-limit response")
	}
}

// TestJsonRpc2Client_GetRaw_GenericError locks the contract that a non-zero
// error code other than -5 surfaces the server message as a plain error.
func TestJsonRpc2Client_GetRaw_GenericError(t *testing.T) {
	f := newFakeSignalCli(t)
	defer f.close()
	number := "+15551230000"

	c := newDialedClient(t, f, number)

	errCh := make(chan error, 1)
	go func() {
		_, err := c.getRaw("send", nil, nil)
		errCh <- err
	}()

	srv := f.accepted(t)
	req := readRequestLine(t, srv)
	id, _ := req["id"].(string)

	deliverResponse(t, c, id, JsonRpc2MessageResponse{
		Id:  id,
		Err: Error{Code: -1, Message: "boom"},
	})

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if _, ok := err.(*RateLimitErrorType); ok {
			t.Errorf("error type = *RateLimitErrorType, want a plain error for code -1")
		}
		if err.Error() != "boom" {
			t.Errorf("error = %q, want %q", err.Error(), "boom")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("getRaw did not return for the generic error response")
	}
}

// TestJsonRpc2Client_ReceiveData_RoutesResponseById locks the inbound response
// routing contract: ReceiveData reads a reply frame off the real net.Conn,
// parses it, and routes it to the channel registered for its id. The channel is
// registered before the reader starts, mirroring getRaw's registration.
func TestJsonRpc2Client_ReceiveData_RoutesResponseById(t *testing.T) {
	f := newFakeSignalCli(t)
	defer f.close()
	number := "+15551230000"

	c := newDialedClient(t, f, number)

	respCh := make(chan JsonRpc2MessageResponse, 1)
	id := "fixed-correlation-id"
	c.receivedResponsesMutex.Lock()
	c.receivedResponsesById[id] = respCh
	c.receivedResponsesMutex.Unlock()

	go c.ReceiveData(number)
	srv := f.accepted(t)
	srv.Write([]byte(`{"id":"` + id + `","result":{"accounts":["` + number + `"]}}` + "\n"))

	select {
	case resp := <-respCh:
		if resp.Id != id {
			t.Errorf("routed id = %q, want %q", resp.Id, id)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(resp.Result, &parsed); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		accounts, ok := parsed["accounts"].([]interface{})
		if !ok || len(accounts) != 1 || accounts[0] != number {
			t.Errorf("routed result accounts = %v, want [%q]", parsed["accounts"], number)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("response was not routed to the registered channel")
	}
}

// TestJsonRpc2Client_ReceiveData_DispatchesReceiveMessages locks the inbound
// capture contract: a server-pushed "receive" frame is parsed off the real
// net.Conn and delivered, with its params intact, to the channel handed out by
// GetReceiveChannel.
func TestJsonRpc2Client_ReceiveData_DispatchesReceiveMessages(t *testing.T) {
	f := newFakeSignalCli(t)
	defer f.close()
	number := "+15551230000"

	c := newDialedClient(t, f, number)
	recvCh, channelUuid, err := c.GetReceiveChannel()
	if err != nil {
		t.Fatalf("GetReceiveChannel: %v", err)
	}
	defer c.RemoveReceiveChannel(channelUuid)

	go c.ReceiveData(number)
	srv := f.accepted(t)

	// ReceiveData dispatches to receive channels with a non-blocking send, so a
	// single frame can be dropped if it lands before the test goroutine is
	// parked on recvCh. Re-send the same frame on a short interval until one is
	// received; extra frames are simply dropped, so the observed contract is
	// unchanged but delivery no longer depends on exact scheduling.
	stop := make(chan struct{})
	go func() {
		frame := []byte(`{"jsonrpc":"2.0","method":"receive","params":{"envelope":{"source":"+15559876543","dataMessage":{"message":"ping"}}}}` + "\n")
		for {
			select {
			case <-stop:
				return
			default:
				srv.Write(frame)
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()
	defer close(stop)

	select {
	case msg := <-recvCh:
		if msg.Method != "receive" {
			t.Errorf("method = %q, want \"receive\"", msg.Method)
		}
		var params struct {
			Envelope struct {
				Source      string `json:"source"`
				DataMessage struct {
					Message string `json:"message"`
				} `json:"dataMessage"`
			} `json:"envelope"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			t.Fatalf("params is not valid JSON: %v", err)
		}
		if params.Envelope.Source != "+15559876543" {
			t.Errorf("envelope.source = %q, want %q", params.Envelope.Source, "+15559876543")
		}
		if params.Envelope.DataMessage.Message != "ping" {
			t.Errorf("envelope.dataMessage.message = %q, want %q", params.Envelope.DataMessage.Message, "ping")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a receive frame was not dispatched to the receive channel")
	}
}
