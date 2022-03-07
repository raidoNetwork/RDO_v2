package main

import (
	"errors"
	"flag"
	"github.com/raidoNetwork/RDO_v2/shared/fileutil"
	"github.com/raidoNetwork/RDO_v2/shared/keystore"
	"log"
	"path/filepath"
)

func main() {
	output := flag.String("o", "", "Path to new key file")
	flag.Parse()

	err := genKey(*output)
	if err != nil {
		log.Panic(err)
	}
}

func genKey(outputPath string) error {
	accman := keystore.NewAccountManager(nil)

	pubKey, err := accman.CreatePair()
	if err != nil {
		return err
	}

	privKey := accman.GetHexPrivateKey(pubKey)

	log.Printf("Generate key:")
	log.Printf("PublicKey: %s", pubKey)

	isPathGiven := len(outputPath) > 0
	if !isPathGiven {
		log.Printf("PrivateKey: %s", privKey)
		return nil
	}

	outputDir := filepath.Dir(outputPath)
	exists, err := fileutil.HasDir(outputDir)
	if err != nil {
		return err
	}

	if !exists {
		return errors.New("given path doesn't exists")
	}

	err = accman.StoreKey(pubKey, outputPath)
	if err != nil {
		return err
	}

	return nil
}
