package keystore

import (
	"crypto/ecdsa"
	"encoding/hex"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/fileutil"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"path/filepath"
	"strconv"
)

const (
	keyDir = "keys"
)

var (
	ErrEmptyStore  = errors.New("Can't save empty store.")
	ErrEmptyKeyDir = errors.New("Key dir is empty.")
)

var log = logrus.WithField("prefix", "keystore")

func NewAccountManager(ctx *cli.Context) *AccountManager {
	am := &AccountManager{
		store:   map[string]*ecdsa.PrivateKey{},
		list:    make([]KeyPair, 0),
		ctx:     ctx,
		dataDir: ctx.String(cmd.DataDirFlag.Name),
	}

	return am
}

type KeyPair struct {
	Address common.Address
	Private *ecdsa.PrivateKey
}

type AccountManager struct {
	store   map[string]*ecdsa.PrivateKey
	list    []KeyPair
	ctx     *cli.Context
	keyPath string
	dataDir string
}

func (am *AccountManager) CreatePair() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", err
	}

	addr := am.createAddress(key)
	hash := addr.Hex()

	if _, exists := am.store[hash]; exists {
		return "", errors.New("key " + hash + " already exists!")
	}

	am.store[hash] = key
	am.list = append(am.list, KeyPair{
		Address: addr,
		Private: key,
	})

	return hash, nil
}

func (am *AccountManager) CreatePairs(n int) error {
	for i := 0; i < n; i++ {
		_, err := am.CreatePair()
		if err != nil {
			return err
		}
	}

	return nil
}

func (am *AccountManager) GetPairs() map[string]*ecdsa.PrivateKey {
	return am.store
}

func (am *AccountManager) GetPairsList() []KeyPair {
	if len(am.list) == 0 || len(am.list) != len(am.store) {
		am.list = make([]KeyPair, 0, len(am.store))

		for hex, priv := range am.store {
			addr := common.HexToAddress(hex)
			am.list = append(am.list, KeyPair{
				Address: addr,
				Private: priv,
			})
		}
	}

	return am.list
}

func (am *AccountManager) GetKey(addr string) *ecdsa.PrivateKey {
	return am.store[addr]
}

func (am *AccountManager) createKeyDir() error {
	keyPath := filepath.Join(am.dataDir, keyDir)

	hasDir, err := fileutil.HasDir(keyPath)
	if err != nil {
		return err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(keyPath); err != nil {
			return err
		}
	}

	am.keyPath = keyPath

	return nil
}

// StoreToDisk saves all created key pairs to the directory with path: {DATADIR}/keys
func (am *AccountManager) StoreToDisk() error {
	if am.keyPath == "" {
		err := am.createKeyDir()
		if err != nil {
			return err
		}
	}

	size := len(am.store)
	if size == 0 {
		return ErrEmptyStore
	}

	i := 0
	for _, key := range am.store {
		fname := "account_" + strconv.Itoa(i)
		path := filepath.Join(am.keyPath, fname)

		err := crypto.SaveECDSA(path, key)
		if err != nil {
			return err
		}

		i++
	}

	return nil
}

func (am *AccountManager) LoadFromDisk() error {
	if am.keyPath == "" {
		err := am.createKeyDir()
		if err != nil {
			return err
		}
	}

	files, err := ioutil.ReadDir(am.keyPath)
	if err != nil {
		return err
	}

	flen := len(files)
	if flen == 0 {
		return ErrEmptyKeyDir
	}

	for _, f := range files {
		// skip directories and UNIX hidden files
		if f.IsDir() || f.Name()[0] == '.' {
			flen--
			continue
		}

		path := filepath.Join(am.keyPath, f.Name())
		key, err := crypto.LoadECDSA(path)
		if err != nil {
			log.Errorf("Error reading file %s", path)
			return err
		}

		addr := am.createAddress(key)
		am.store[addr.Hex()] = key
	}

	if flen == 0 {
		return ErrEmptyKeyDir
	}

	return nil
}

func (am *AccountManager) createAddress(key *ecdsa.PrivateKey) common.Address {
	return crypto.PubkeyToAddress(key.PublicKey)
}

func (am *AccountManager) GetHexPrivateKey(pubKey string) string {
	key := am.GetKey(pubKey)
	if key == nil {
		return ""
	}

	return "0x" + hex.EncodeToString(crypto.FromECDSA(key))
}

func (am *AccountManager) StoreKey(addr string, path string) error {
	key := am.GetKey(addr)
	if key == nil {
		return errors.New("Undefined key")
	}

	err := crypto.SaveECDSA(path, key)
	if err != nil {
		return err
	}

	return nil
}