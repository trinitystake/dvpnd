// SPDX-License-Identifier: Apache-2.0

// e2e drives a running dvpnd from the client side: it buys a pay-per-session
// slot on the node with the given key, performs the root-path handshake the way
// current client apps do, and writes a WireGuard client configuration for the
// resulting peer. Run it against a node started from the same home directory:
//
//	go run ./tools/e2e -home ~/.dvpnd-test -api https://127.0.0.1:8585
//
// It spends real funds (the session deposit plus gas) on the chain the node is
// configured for.
package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	base "github.com/sentinel-official/sentinelhub/v12/types"
	v1base "github.com/sentinel-official/sentinelhub/v12/types/v1"
	nodetypes "github.com/sentinel-official/sentinelhub/v12/x/node/types/v3"
	"github.com/spf13/viper"

	"github.com/trinitystake/dvpnd/lite"
	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
	"github.com/trinitystake/dvpnd/types"
)

func main() {
	var (
		home       = flag.String("home", types.DefaultHomeDirectory, "node home directory (config.toml, keyring)")
		api        = flag.String("api", "https://127.0.0.1:8585", "node API base URL")
		gigabytes  = flag.Int64("gigabytes", 1, "session size to buy")
		maxPrice   = flag.String("max-price", "", "maximum price accepted (chain notation denom:base,quote); default: the node's own udvpn gigabyte price")
		sessionID  = flag.Uint64("session", 0, "reuse an existing session id instead of buying one")
		deactivate = flag.Bool("deactivate", false, "only send MsgUpdateNodeStatus(inactive) for this key's node and exit")
		out        = flag.String("out", "", "where to write the WireGuard client config (default <home>/e2e-client.conf)")
		endpoint   = flag.String("endpoint", "127.0.0.1", "host written as the WireGuard endpoint in the client config")
		fullTunnel = flag.Bool("full-tunnel", false, "route all client traffic through the node (0.0.0.0/0, ::/0) instead of only its tunnel address")
	)
	flag.Parse()
	if err := run(*home, *api, *gigabytes, *maxPrice, *sessionID, *out, *deactivate, *endpoint, *fullTunnel); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(home, api string, gigabytes int64, maxPriceStr string, sessionID uint64, out string, deactivate bool, endpoint string, fullTunnel bool) error {
	base.GetConfig().Seal()

	v := viper.New()
	v.SetConfigFile(filepath.Join(home, types.ConfigFileName))
	config, err := types.ReadInConfig(v)
	if err != nil {
		return err
	}

	kr, err := keyring.New(types.KeyringName, config.Keyring.Backend, home, bufio.NewReader(os.Stdin), lite.DefaultEncodingConfig().Codec)
	if err != nil {
		return err
	}
	record, err := kr.Key(config.Keyring.From)
	if err != nil {
		return err
	}
	accAddr, err := record.GetAddress()
	if err != nil {
		return err
	}
	nodeAddr := base.NodeAddress(accAddr.Bytes())
	fmt.Printf("account %s\nnode    %s\n", accAddr, nodeAddr)

	client := lite.NewDefaultClient().
		WithChainID(config.Chain.ID).
		WithFromAddress(accAddr).
		WithFromName(config.Keyring.From).
		WithGas(config.Chain.Gas).
		WithGasAdjustment(config.Chain.GasAdjustment).
		WithGasPrices(config.Chain.GasPrices).
		WithKeyring(kr).
		WithLogger(cmtlog.NewNopLogger()).
		WithQueryTimeout(config.Chain.RPCQueryTimeout).
		WithRemotes(strings.Split(config.Chain.RPCAddresses, ",")).
		WithSignModeStr("").
		WithSimulateAndExecute(config.Chain.SimulateAndExecute).
		WithTxTimeout(config.Chain.RPCTxTimeout)

	if deactivate {
		res, err := client.Tx(nodetypes.NewMsgUpdateNodeStatusRequest(nodeAddr, v1base.StatusInactive))
		if err != nil {
			return fmt.Errorf("MsgUpdateNodeStatus: %w", err)
		}
		fmt.Printf("node set inactive: tx %s height %d\n", res.TxHash, res.Height)
		return nil
	}

	// 1. The node must be registered and active before a session can start.
	node, err := client.QueryNode(nodeAddr)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("node %s is not registered; start dvpnd first", nodeAddr)
	}
	fmt.Printf("on-chain node: status=%s remote_addrs=%v gigabyte_prices=%s\n", node.Status, node.RemoteAddrs, node.GetGigabytePrices())
	if !node.Status.Equal(v1base.StatusActive) {
		return fmt.Errorf("node status is %s; wait for the first status update", node.Status)
	}

	// 2. GET / — what client apps read first.
	http.DefaultClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	http.DefaultClient.Timeout = 15 * time.Second
	root, err := http.Get(strings.TrimRight(api, "/") + "/")
	if err != nil {
		return fmt.Errorf("GET /: %w", err)
	}
	rootBody, _ := io.ReadAll(root.Body)
	root.Body.Close()
	fmt.Printf("GET / -> %d %s\n", root.StatusCode, truncate(rootBody, 300))

	// 3. Buy a session on the node (pay-per-session, node module).
	if sessionID == 0 {
		var maxPrice v1base.Price
		if maxPriceStr != "" {
			if maxPrice, err = v1base.NewPriceFromString(maxPriceStr); err != nil {
				return fmt.Errorf("max-price: %w", err)
			}
		} else {
			price, found := node.GigabytePrice("udvpn")
			if !found {
				return fmt.Errorf("node has no udvpn gigabyte price; pass -max-price")
			}
			maxPrice = price
		}
		fmt.Printf("buying %d GB at up to %s (deposit %s udvpn)\n", gigabytes, maxPrice, maxPrice.QuoteValue.MulRaw(gigabytes))
		res, err := client.Tx(nodetypes.NewMsgStartSessionRequest(accAddr, nodeAddr, gigabytes, 0, maxPrice))
		if err != nil {
			return fmt.Errorf("MsgStartSession: %w", err)
		}
		fmt.Printf("MsgStartSession included: tx %s height %d\n", res.TxHash, res.Height)

		sessions, err := client.QuerySessionsForAccount(accAddr)
		if err != nil {
			return err
		}
		for _, s := range sessions {
			if s.GetNodeAddress() == nodeAddr.String() && s.GetStatus().Equal(v1base.StatusActive) && s.GetID() > sessionID {
				sessionID = s.GetID()
			}
		}
		if sessionID == 0 {
			return fmt.Errorf("no active session for %s on %s after the transaction", accAddr, nodeAddr)
		}
	}
	fmt.Printf("session id %d\n", sessionID)

	// 4. Handshake exactly like current client apps: sign BE64(id) || raw JSON.
	wgKey, err := wgtypes.NewPrivateKey()
	if err != nil {
		return err
	}
	peerJSON, _ := json.Marshal(map[string]string{"public_key": wgKey.Public().String()})
	sig, pubKey, err := kr.Sign(config.Keyring.From, append(sdk.Uint64ToBigEndian(sessionID), peerJSON...))
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]interface{}{
		"data":      base64.StdEncoding.EncodeToString(peerJSON),
		"id":        sessionID,
		"pub_key":   "secp256k1:" + base64.StdEncoding.EncodeToString(pubKey.Bytes()),
		"signature": base64.StdEncoding.EncodeToString(sig),
	})
	resp, err := http.Post(strings.TrimRight(api, "/")+"/", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST /: %w", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("POST / -> %d %s\n", resp.StatusCode, truncate(respBody, 400))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("handshake refused")
	}

	var envelope struct {
		Result struct {
			Data  string   `json:"data"`
			Addrs []string `json:"addrs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return err
	}
	dataJSON, err := base64.StdEncoding.DecodeString(envelope.Result.Data)
	if err != nil {
		return err
	}
	var data struct {
		Addrs    []string `json:"addrs"`
		Metadata []struct {
			Port      uint16 `json:"port"`
			PublicKey string `json:"public_key"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		return err
	}
	if len(data.Metadata) == 0 || len(data.Addrs) == 0 || len(envelope.Result.Addrs) == 0 {
		return fmt.Errorf("handshake data incomplete: %s", dataJSON)
	}
	fmt.Printf("handshake data: addrs=%v port=%d node_pubkey=%s endpoint_hosts=%v\n",
		data.Addrs, data.Metadata[0].Port, data.Metadata[0].PublicKey, envelope.Result.Addrs)

	// 5. A client config. By default it routes only the node's tunnel address, so
	// it can be brought up on the same machine without hijacking its default
	// route; -full-tunnel routes everything through the node and belongs on a
	// separate host or container.
	if out == "" {
		out = filepath.Join(home, "e2e-client.conf")
	}
	allowedIPs := "10.8.0.1/32"
	if fullTunnel {
		allowedIPs = "0.0.0.0/0, ::/0"
	}
	conf := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s

[Peer]
PublicKey = %s
Endpoint = %s:%d
AllowedIPs = %s
PersistentKeepalive = 15
`, wgKey.String(), strings.Join(data.Addrs, ","), data.Metadata[0].PublicKey, endpoint, data.Metadata[0].Port, allowedIPs)
	if err := os.WriteFile(out, []byte(conf), 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s (endpoint %s:%d, allowed ips %s)\n", out, endpoint, data.Metadata[0].Port, allowedIPs)
	fmt.Printf("next (as root): wg-quick up %s && sleep 20 && wg show && wg-quick down %s\n", out, out)
	if fullTunnel {
		fmt.Println("proof: 'curl https://1.1.1.1/cdn-cgi/trace' on the client answers through the node and both 'wg show' outputs count the bytes.")
	} else {
		fmt.Println("proof: a 'latest handshake' line and non-zero 'transfer' on both interfaces. (Pinging 10.8.0.1 from the")
		fmt.Println("same host answers locally and proves nothing.) The node reports usage on-chain at its next update_sessions tick.")
	}

	return nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
