package vpnconfig

import (
	"fmt"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/tyrax/tyrax-backend/internal/model"
)

// GenerateClientConfig builds a ready-to-import WireGuard .conf for a single
// device. clientPublicKey is part of the device identity registered server-side
// and is therefore not written into the client-facing file.
func GenerateClientConfig(serverNode model.Node, clientPrivateKey, clientPublicKey, serverPublicKey, clientIP string) string {
	_ = clientPublicKey // registered on the node, not emitted in the client config

	var b strings.Builder

	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", clientPrivateKey)
	fmt.Fprintf(&b, "Address = %s/32\n", clientIP)
	b.WriteString("DNS = 1.1.1.1, 8.8.8.8\n")
	b.WriteString("MTU = 1420\n")
	b.WriteString("\n")
	b.WriteString("[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", serverPublicKey)
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", serverNode.Host, serverNode.Port)
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	b.WriteString("PersistentKeepalive = 25\n")

	return b.String()
}

// GenerateKeypair creates a fresh Curve25519 keypair and returns the private and
// derived public keys as base64 strings (WireGuard's wire format).
func GenerateKeypair() (privateKey, publicKey string, err error) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", fmt.Errorf("generate wireguard keypair: %w", err)
	}
	return key.String(), key.PublicKey().String(), nil
}
