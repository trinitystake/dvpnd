// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package wireguard

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/viper"

	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
	"github.com/trinitystake/dvpnd/types"
)

const (
	InfoLen = 2 + 32
)

var (
	_ types.Service = (*WireGuard)(nil)
)

type WireGuard struct {
	info   []byte
	config *wgtypes.Config
	peers  *wgtypes.Peers
	pool   *wgtypes.IPPool
}

func NewWireGuard(pool *wgtypes.IPPool) types.Service {
	return &WireGuard{
		pool:   pool,
		config: wgtypes.NewConfig(),
		info:   make([]byte, InfoLen),
		peers:  wgtypes.NewPeers(),
	}
}

func (s *WireGuard) Type() uint64 {
	return wgtypes.Type
}

func (s *WireGuard) Init(home string) (err error) {
	v := viper.New()
	v.SetConfigFile(filepath.Join(home, wgtypes.ConfigFileName))

	s.config, err = wgtypes.ReadInConfig(v)
	if err != nil {
		return err
	}
	if err = s.config.Validate(); err != nil {
		return err
	}

	if s.config.Uplink == "" {
		s.config.Uplink = detectUplink()
	}

	t, err := template.New("wireguard_conf").Parse(configTemplate)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	if err = t.Execute(&buffer, s.config); err != nil {
		return err
	}

	path := fmt.Sprintf("/etc/wireguard/%s.conf", s.config.Interface)
	if err = os.WriteFile(path, buffer.Bytes(), 0600); err != nil {
		return err
	}

	key, err := wgtypes.KeyFromString(s.config.PrivateKey)
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint16(s.info[:2], s.config.ListenPort)
	copy(s.info[2:], key.Public().Bytes())

	return nil
}

func (s *WireGuard) Info() []byte {
	return s.info
}

// detectUplink returns the interface of the default IPv4 route, which is where
// peer traffic must be masqueraded. Falls back to eth0, upstream's fixed choice.
func detectUplink() string {
	out, err := exec.Command("ip", "-o", "-4", "route", "show", "default").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "dev" {
				return fields[i+1]
			}
		}
	}

	return "eth0"
}

func (s *WireGuard) wgQuick(action string) error {
	cmd := exec.Command("wg-quick", action, s.config.Interface)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// forwardingSwitches are the kernel settings peer traffic depends on. IPv4 is
// mandatory; IPv6 is best-effort because a host or container without IPv6 has
// no such file.
type forwardingSwitch struct {
	path     string
	name     string
	required bool
}

var forwardingSwitches = []forwardingSwitch{
	{"/proc/sys/net/ipv4/ip_forward", "net.ipv4.ip_forward", true},
	{"/proc/sys/net/ipv6/conf/all/forwarding", "net.ipv6.conf.all.forwarding", false},
}

// ensureForwarding turns IP forwarding on unless it already is. It reads before
// writing because /proc/sys is read-only inside an unprivileged container: there
// the value must come from the runtime (docker run --sysctl net.ipv4.ip_forward=1)
// and a blind write would fail even though the setting is right.
func ensureForwarding() error {
	for _, sw := range forwardingSwitches {
		if cur, err := os.ReadFile(sw.path); err == nil && strings.TrimSpace(string(cur)) == "1" {
			continue
		}

		err := os.WriteFile(sw.path, []byte("1\n"), 0o644)
		if err == nil {
			continue
		}
		if sw.required {
			return fmt.Errorf("%s is off and cannot be enabled (%v); set it on the host, or pass --sysctl %s=1 to the container runtime", sw.name, err, sw.name)
		}

		fmt.Fprintf(os.Stderr, "warning: %s could not be enabled (%v); IPv6 peer traffic will not be forwarded\n", sw.name, err)
	}

	return nil
}

// Start brings the interface up. An earlier instance that died without Stop
// leaves the interface behind and wg-quick refuses to create it again; in that
// case it is torn down and recreated so a restart needs no manual cleanup.
func (s *WireGuard) Start() error {
	if err := ensureForwarding(); err != nil {
		return err
	}

	err := s.wgQuick("up")
	if err == nil {
		return nil
	}

	if _, statErr := os.Stat(filepath.Join("/sys/class/net", s.config.Interface)); statErr != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "interface %s already exists; recreating it\n", s.config.Interface)
	_ = s.wgQuick("down")

	return s.wgQuick("up")
}

func (s *WireGuard) Stop() error {
	return s.wgQuick("down")
}

func (s *WireGuard) AddPeer(data []byte) (result []byte, err error) {
	identity := base64.StdEncoding.EncodeToString(data)

	v4, v6, err := s.pool.Get()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			s.pool.Release(v4, v6)
		}
	}()

	// With IPv6 off the peer gets only an IPv4 tunnel address: the v6 slot in the
	// result stays zero, which tells the handshake to hand the client just the
	// IPv4 address, so it routes nothing over IPv6 through the tunnel.
	allowedIPs := fmt.Sprintf("%s/32", v4.IP())
	if s.config.EnableIPv6 {
		allowedIPs += fmt.Sprintf(",%s/128", v6.IP())
	} else {
		s.pool.V6.Release(v6)
		v6 = wgtypes.IPv6{}
	}

	cmd := exec.Command("wg", strings.Split(
		fmt.Sprintf(`set %s peer %s allowed-ips %s`,
			s.config.Interface, identity, allowedIPs), " ")...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		return nil, err
	}

	s.peers.Put(
		wgtypes.Peer{
			Identity: identity,
			IPv4:     v4,
			IPv6:     v6,
		},
	)

	result = append(result, v4.Bytes()...)
	result = append(result, v6.Bytes()...)
	return result, nil
}

func (s *WireGuard) HasPeer(data []byte) bool {
	var (
		identity = base64.StdEncoding.EncodeToString(data)
		peer     = s.peers.Get(identity)
	)

	return !peer.Empty()
}

func (s *WireGuard) RemovePeer(data []byte) error {
	identity := base64.StdEncoding.EncodeToString(data)

	cmd := exec.Command("wg", strings.Split(
		fmt.Sprintf(`set %s peer %s remove`,
			s.config.Interface, identity), " ")...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	if v := s.peers.Get(identity); !v.Empty() {
		s.peers.Delete(v.Identity)
		s.pool.Release(v.IPv4, v.IPv6)
	}

	return nil
}

func (s *WireGuard) Peers() (items []types.Peer, err error) {
	output, err := exec.Command("wg", strings.Split(
		fmt.Sprintf("show %s transfer", s.config.Interface), " ")...).Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		columns := strings.Split(line, "\t")
		if len(columns) != 3 {
			continue
		}

		upload, err := strconv.ParseInt(columns[1], 10, 64)
		if err != nil {
			return nil, err
		}

		download, err := strconv.ParseInt(columns[2], 10, 64)
		if err != nil {
			return nil, err
		}

		items = append(items,
			types.Peer{
				Key:      columns[0],
				Upload:   upload,
				Download: download,
			},
		)
	}

	return items, nil
}

func (s *WireGuard) PeerCount() int {
	return s.peers.Len()
}
