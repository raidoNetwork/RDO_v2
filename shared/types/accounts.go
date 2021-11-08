package types

import (
	"crypto/ecdsa"
	"encoding/hex"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"path/filepath"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/fileutil"
	"strconv"
)

const (
	keyDir = "keys"
)

var (
	ErrEmptyStore  = errors.New("Can't save empty store.")
	ErrEmptyKeyDir = errors.New("Key dir is empty.")
)

func NewAccountManager(ctx *cli.Context) (*AccountManager, error) {
	am := &AccountManager{
		store: map[string]*ecdsa.PrivateKey{},
		ctx:   ctx,
	}

	baseDir := am.ctx.String(cmd.DataDirFlag.Name)
	keyPath := filepath.Join(baseDir, keyDir)

	hasDir, err := fileutil.HasDir(keyPath)
	if err != nil {
		return nil, err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(keyPath); err != nil {
			return nil, err
		}
	}

	am.keyPath = keyPath

	return am, nil
}

type KeyPair struct {
	Address string
	Private *ecdsa.PrivateKey
}

type AccountManager struct {
	store   map[string]*ecdsa.PrivateKey
	ctx     *cli.Context
	keyPath string
}

func (am *AccountManager) CreatePair() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", err
	}

	hash := getPubKeyHex(key)

	if _, exists := am.store[hash]; exists {
		return "", errors.New("key " + hash + " already exists!")
	}

	am.store[hash] = key

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
	data := make([]KeyPair, 0, len(am.store))

	for hash, priv := range am.store {
		data = append(data, KeyPair{
			Address: hash,
			Private: priv,
		})
	}

	return data
}

func (am *AccountManager) GetKey(addr string) *ecdsa.PrivateKey {
	return am.store[addr]
}

// StoreToDisk saves all created key pairs to the directory with path: {DATADIR}/keys
func (am *AccountManager) StoreToDisk() error {
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
	files, err := ioutil.ReadDir(am.keyPath)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return ErrEmptyKeyDir
	}

	for _, f := range files {
		path := filepath.Join(am.keyPath, f.Name())
		key, err := crypto.LoadECDSA(path)
		if err != nil {
			return err
		}

		pubKey := getPubKeyHex(key)
		am.store[pubKey] = key
	}

	return nil
}

func getPubKeyHex(key *ecdsa.PrivateKey) string {
	return hex.EncodeToString(crypto.CompressPubkey(&key.PublicKey)[1:])
}
