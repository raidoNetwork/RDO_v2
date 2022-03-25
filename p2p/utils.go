package p2p

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/pkg/errors"
	rcrypto "github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/utils/file"
	"net"
	"path"
)

const keyPath = "node-keys"
const keyName = "node.key"

type nodeKey struct {
	p2pKey crypto.PrivKey
}

func getNodeKey(dataPath string) (*nodeKey, error) {
	if dataPath == "" {
		return nil, errors.New("empty data path")
	}

	var key crypto.PrivKey
	var err error

	nodeKeyPath := path.Join(dataPath, keyPath)
	dirExists, err := file.HasDir(nodeKeyPath)
	if err != nil {
		return nil, err
	}

	if !dirExists {
		if err := file.MkdirAll(nodeKeyPath); err != nil {
			return nil, err
		}
	}

	keyFile := path.Join(nodeKeyPath, keyName)
	exists := file.FileExists(keyFile)
	if !exists {
		key, _, err = crypto.GenerateEd25519Key(nil)
		if err != nil {
			return nil, err
		}

		err = rcrypto.SaveP2PKey(keyFile, key)
		if err != nil {
			return nil, err
		}
	} else {
		key, err = rcrypto.LoadP2PKey(keyFile)
		if err != nil {
			return nil, err
		}
	}

	nKey := &nodeKey{
		p2pKey: key,
	}

	return nKey, nil
}

func getIPaddr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var ipAddrs []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// skip invalid addresses
			// save only IPv4
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.To4() == nil {
				continue
			}

			ipAddrs = append(ipAddrs, ip)
		}
	}

	// return localhost
	if len(ipAddrs) == 0 {
		return "127.0.0.1", nil
	}

	return ipAddrs[0].String(), nil
}
