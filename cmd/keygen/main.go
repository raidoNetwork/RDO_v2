package main

import (
	"github.com/raidoNetwork/RDO_v2/shared/keystore"
	"log"
)

func main() {
	err := genKey()
	if err != nil {
		log.Panic(err)
	}
}

func genKey() error {
	accman, err := keystore.NewAccountManager(nil)
	if err != nil {
		return err
	}

	pubKey, err := accman.CreatePair()
	if err != nil {
		return err
	}

	privKey := accman.GetHexPrivateKey(pubKey)

	log.Printf("PublicKey: %s", pubKey)
	log.Printf("PrivateKey: %s", privKey)

	return nil
}
