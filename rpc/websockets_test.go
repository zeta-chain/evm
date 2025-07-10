package rpc

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	rpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"

	"github.com/cosmos/evm/server/config"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client"
)

func newTestWebsocketServer() *websocketsServer {
	// dummy values for testing
	cfg := &config.Config{}
	cfg.JSONRPC.Address = "localhost:9999"   // not used
	cfg.JSONRPC.WsAddress = "localhost:9999" // not used
	cfg.TLS.CertificatePath = ""
	cfg.TLS.KeyPath = ""

	return &websocketsServer{
		rpcAddr:  cfg.JSONRPC.Address,
		wsAddr:   cfg.JSONRPC.WsAddress,
		certFile: cfg.TLS.CertificatePath,
		keyFile:  cfg.TLS.KeyPath,
		api:      newPubSubAPI(client.Context{}, log.NewNopLogger(), &rpcclient.WSClient{}),
		logger:   log.NewNopLogger(),
	}
}

func TestWebsocketPayloadLimit(t *testing.T) {
	srv := newTestWebsocketServer()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(u.String(), nil)
	require.NoError(t, err)

	defer conn.Close()

	// Send oversized message (2 MB)
	oversizedPayload := make([]byte, 2<<20)
	_ = conn.WriteMessage(websocket.TextMessage, oversizedPayload)

	// The connection should close
	_, _, readErr := conn.ReadMessage()
	require.Error(t, readErr, "expected connection to close on oversized message")
}
