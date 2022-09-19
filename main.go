package main

import (
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/ecdsa/keygen"
	"github.com/binance-chain/tss-lib/ecdsa/signing"
	"github.com/binance-chain/tss-lib/test"
	"github.com/binance-chain/tss-lib/tss"
	"io/fs"
	"log"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	curve     elliptic.Curve
	count     int
	threshold int

	dataDir string
)

func main() {
	// setup
	dataDir = "./data/"
	if strings.ToLower(os.Getenv("CURVE")) == "ed25519" {
		curve = tss.Edwards()
		log.Printf("Curve: ed25519\n")
	} else {
		curve = tss.S256()
		log.Printf("Curve: secp256k1\n")
	}
	count, _ = strconv.Atoi(os.Getenv("COUNT"))
	threshold, _ = strconv.Atoi(os.Getenv("THRESHOLD"))
	log.Printf("Quorum: %d/%d", threshold, count)

	// get ids for parties
	partyIDs := tss.GenerateTestPartyIDs(count)

	// use gg18 dkg
	dirEntries, _ := os.ReadDir(dataDir)
	if len(dirEntries) < count {
		log.Printf("Keygen protocol executing")
		_ = os.RemoveAll(dataDir)
		_ = execKeygen(partyIDs)
	} else {
		log.Printf("Keygen data found")
	}
	keygenSaves, partyIDs, err := loadKeygenSaves()

	// use gg18 tss
	message := big.NewInt(int64(rand.Int()))
	signature, err := execSigning(message, partyIDs, keygenSaves)
	if err != nil {
		log.Fatalf("Error occurred in execSigning: %+v\n", err)
	}

	fmt.Printf("\nMessage: %s\n", message.String())
	fmt.Printf("Public key (compressed): %s\n", keygenSaves[0].ECDSAPub.X().String())
	fmt.Printf("Signature: %x\n", string(signature))
}

func execKeygen(partyIDs tss.SortedPartyIDs) []keygen.LocalPartySaveData {
	ctx := tss.NewPeerContext(partyIDs)
	parties := make([]*keygen.LocalParty, count)

	// communication channel setup
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(partyIDs))
	errCh := make(chan *tss.Error, len(partyIDs))

	// initialize parties
	for i := 0; i < len(partyIDs); i++ {
		preParams, _ := keygen.GeneratePreParams(1 * time.Minute)
		params := tss.NewParameters(curve, ctx, partyIDs[i], len(partyIDs), threshold)
		party := keygen.NewLocalParty(params, outCh, endCh, *preParams).(*keygen.LocalParty)
		partyIndex := party.PartyID().Index
		parties[partyIndex] = party
		go func(party *keygen.LocalParty) {
			if err := party.Start(); err != nil {
				errCh <- err
			}
		}(party)
	}

	// execute protocol
	var output []keygen.LocalPartySaveData
	for { // continuous loop
		select {
		case err := <-errCh:
			log.Fatalf("Error occurred during protocol execution: %+v\n", err)
		case msg := <-outCh:
			log.Printf("Out channel received message: %+v\n", msg.String())
			recipients := msg.GetTo()
			if recipients == nil { // broadcast
				for _, party := range parties {
					go test.SharedPartyUpdater(party, msg, errCh)
				}
			} else { // p2p
				go test.SharedPartyUpdater(parties[recipients[0].Index], msg, errCh)
			}
		case save := <-endCh:
			index, err := save.OriginalIndex()
			if err != nil {
				log.Fatalf("Error occurred in save.OriginalIndex: %+v\n", err)
			}

			data, _ := json.Marshal(&save)
			path, _ := filepath.Abs("./data/" + strconv.Itoa(index) + "_save.json")
			if _, err = os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				_ = os.MkdirAll(filepath.Dir(path), fs.ModePerm)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				log.Fatalf("Error occurred in os.WriteFile: %+v\n", err)
			}
			log.Printf("Keygen data saved (%s).", path)

			if output = append(output, save); len(output) == len(parties) {
				return output
			}
		}
	}
}

func execSigning(message *big.Int, partyIDs []*tss.PartyID, keygenSaves []keygen.LocalPartySaveData) ([]byte, error) {
	ctx := tss.NewPeerContext(partyIDs)
	parties := make([]*signing.LocalParty, count)

	// communication channel setup
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan common.SignatureData, len(partyIDs))
	errCh := make(chan *tss.Error, len(partyIDs))

	// initialize parties
	for i := 0; i < len(partyIDs); i++ {
		params := tss.NewParameters(curve, ctx, partyIDs[i], len(partyIDs), threshold)
		party := signing.NewLocalParty(message, params, keygenSaves[i], outCh, endCh).(*signing.LocalParty)
		partyIndex := party.PartyID().Index
		parties[partyIndex] = party
		go func(party *signing.LocalParty) {
			if err := party.Start(); err != nil {
				errCh <- err
			}
		}(party)
	}

	// execute protocol
	for { // continuous loop
		select {
		case err := <-errCh:
			log.Fatalf("Error occurred during protocol execution: %+v\n", err)
			return nil, err
		case msg := <-outCh:
			log.Printf("Out channel received message: %+v\n", msg.String())
			recipients := msg.GetTo()
			if recipients == nil { // broadcast
				for _, party := range parties {
					go test.SharedPartyUpdater(party, msg, errCh)
				}
			} else { // p2p
				go test.SharedPartyUpdater(parties[recipients[0].Index], msg, errCh)
			}
		case <-endCh:
			log.Printf("Signature computed.")
			return (<-endCh).Signature, nil
		}
	}
}

func loadKeygenSaves() ([]keygen.LocalPartySaveData, tss.SortedPartyIDs, error) {
	keygenSaves := make([]keygen.LocalPartySaveData, 0, count)
	for i := 0; i < count; i++ {
		path, _ := filepath.Abs("./data/" + strconv.Itoa(i) + "_save.json")
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("Error occurred in os.ReadFile: %+v\n", err)
		}

		var keygenSave keygen.LocalPartySaveData
		if err = json.Unmarshal(data, &keygenSave); err != nil {
			log.Fatalf("Error occurred in json.Unmarshal: %+v\n", err)
		}

		for _, kbxj := range keygenSave.BigXj {
			kbxj.SetCurve(curve)
		}
		keygenSave.ECDSAPub.SetCurve(tss.S256())
		keygenSaves = append(keygenSaves, keygenSave)
	}

	partyIDs := make(tss.UnSortedPartyIDs, len(keygenSaves))
	for i, keygenSave := range keygenSaves {
		pMoniker := fmt.Sprintf("%d", i+1)
		partyIDs[i] = tss.NewPartyID(pMoniker, pMoniker, keygenSave.ShareID)
	}
	sortedPIDs := tss.SortPartyIDs(partyIDs)
	return keygenSaves, sortedPIDs, nil
}
