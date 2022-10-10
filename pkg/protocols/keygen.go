package protocols

import (
	"bufio"
	"codis/pkg/log"
	"codis/pkg/utils"
	"codis/proto/pb"
	"context"
	"encoding/json"
	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"io"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/bnb-chain/tss-lib/tss"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	KeygenPID              = "/keygen/0.0.1"
	keygenStepDirectPID    = "/keygen/step/direct/0.0.1"
	keygenStepBroadcastPID = "/keygen/step/broadcast/0.0.1"
)

type KeygenService struct {
	Host libhost.Host

	localParty *keygen.LocalParty
	partyIDMap map[peer.ID]*tss.PartyID

	outCh chan tss.Message
	endCh chan keygen.LocalPartySaveData
	errCh chan *tss.Error

	logger *log.Logger
}

func NewKeygenService(h libhost.Host) *KeygenService {
	logger := log.NewLogger()
	ks := &KeygenService{
		Host: h,

		localParty: &keygen.LocalParty{},
		partyIDMap: make(map[peer.ID]*tss.PartyID),

		outCh: make(chan tss.Message, 1),
		endCh: make(chan keygen.LocalPartySaveData, 1),
		errCh: make(chan *tss.Error, 1),

		logger: logger,
	}

	h.SetStreamHandler(keygenStepDirectPID, ks.keygenStepHandlerDirect)
	logger.Debug("keygen step (direct) sub-protocol stream handler set")

	h.SetStreamHandler(keygenStepBroadcastPID, ks.keygenStepHandlerBroadcast)
	logger.Debug("keygen step (broadcast) sub-protocol stream handler set")

	h.SetStreamHandler(KeygenPID, ks.keygenHandler)
	logger.Debug("keygen protocol stream handler set")

	return ks
}

func (ks *KeygenService) Keygen(ctx context.Context, args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	peerIds, err := utils.AddrStringsToPeerIds(args.Party)
	if err != nil {
		return err
	}

	for _, id := range peerIds {
		if id.String() == ks.Host.ID().String() {
			continue
		}

		err = func() error {
			s, err := ks.Host.NewStream(ctx, id, KeygenPID)
			if err != nil {
				return err
			} else {
				ks.logger.Debug("new stream created with peer %s", id.String())
			}
			defer s.Close()

			msg, err := proto.Marshal(args)
			if err != nil {
				return err
			}

			writer := bufio.NewWriter(s)
			if _, err = writer.Write(msg); err != nil {
				_ = s.Reset()
				return err
			}
			if err := writer.Flush(); err != nil {
				_ = s.Reset()
				return err
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	go ks.keygen(args)
	return nil
}

func (ks *KeygenService) keygenHandler(s network.Stream) {
	var args pb.KeygenArgs
	data, err := io.ReadAll(s)
	if err != nil {
		ks.logger.Error(err)
		return
	}
	err = proto.Unmarshal(data, &args)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	go ks.keygen(&args)
}

func (ks *KeygenService) keygenStepHandlerDirect(s network.Stream) {
	ks.keygenStepHandlerCommon(s, false)
}

func (ks *KeygenService) keygenStepHandlerBroadcast(s network.Stream) {
	ks.keygenStepHandlerCommon(s, true)
}

func (ks *KeygenService) keygenStepHandlerCommon(s network.Stream, broadcast bool) {
	data, err := io.ReadAll(s)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	ok, err := ks.localParty.UpdateFromBytes(data, ks.partyIDMap[s.Conn().RemotePeer()], broadcast)
	if !ok {
		ks.errCh <- ks.localParty.WrapError(err)
	}
}

func (ks *KeygenService) keygen(args *pb.KeygenArgs) {
	ks.logger.Info("keygen started!")
	ks.logger.Info("count: %d, threshold: %d, party: %s", args.Count, args.Threshold, args.Party)

	peerIds, err := utils.AddrStringsToPeerIds(args.Party)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	var thisPartyID *tss.PartyID
	var unsortedPartyIDs tss.UnSortedPartyIDs
	for _, id := range peerIds {
		key := new(big.Int)
		key.SetBytes([]byte(id))
		partyID := tss.NewPartyID(id.String(), "", key)
		if partyID.Id == ks.Host.ID().String() {
			thisPartyID = partyID
		}

		ks.partyIDMap[id] = partyID
		unsortedPartyIDs = append(unsortedPartyIDs, partyID)
	}
	sortedPartyIDs := tss.SortPartyIDs(unsortedPartyIDs)

	preParams, err := keygen.GeneratePreParams(1 * time.Minute)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	peerCtx := tss.NewPeerContext(sortedPartyIDs)
	curve := tss.S256()
	params := tss.NewParameters(curve, peerCtx, thisPartyID, len(sortedPartyIDs), int(args.Threshold))
	ks.localParty = keygen.NewLocalParty(params, ks.outCh, ks.endCh, *preParams).(*keygen.LocalParty)

	time.Sleep(time.Minute * 1)

	go func(localParty *keygen.LocalParty, errCh chan *tss.Error) {
		if err := localParty.Start(); err != nil {
			errCh <- err
		}
	}(ks.localParty, ks.errCh)

	for {
		select {
		case err := <-ks.errCh:
			ks.logger.Error(err)
			return
		case msg := <-ks.outCh:
			ks.logger.Debug("out-channel received message: %s", msg.String())
			msgBytes, msgRouting, err := msg.WireBytes()
			if err != nil {
				ks.logger.Error(err)
				return
			} else if msgRouting.IsBroadcast { // use step broadcast sub-protocol
				for peerID, _ := range ks.partyIDMap {
					if peerID == ks.Host.ID() {
						continue
					}
					if err = stepKeygen(ks.Host, peerID, msgBytes, true); err != nil {
						ks.logger.Error(err)
						return
					}
				}
			} else { // use step direct sub-protocol
				recipients := msg.GetTo()
				for _, recipient := range recipients {
					for peerID, partyID := range ks.partyIDMap {
						if recipient == partyID {
							if err := stepKeygen(ks.Host, peerID, msgBytes, false); err != nil {
								ks.logger.Error(err)
								return
							}
						}
					}
				}
			}
		case save := <-ks.endCh:
			index, err := save.OriginalIndex()
			if err != nil {
				ks.logger.Error(err)
			}

			data, err := json.Marshal(&save)
			if err != nil {
				ks.logger.Error(err)
			}

			saveDir, _ := filepath.Abs("data/")
			if _, err = os.Stat(saveDir); os.IsNotExist(err) {
				_ = os.MkdirAll(filepath.Dir(saveDir), fs.ModePerm)
			}

			saveFile, _ := filepath.Abs(saveDir + "/keysave_" + strconv.Itoa(index) + ".json")
			if err = os.WriteFile(saveFile, data, 0600); err != nil {
				ks.logger.Error(err)
				return
			}
			ks.logger.Info("key file saved")
		}
	}
}

func stepKeygen(host libhost.Host, peer peer.ID, msg []byte, broadcast bool) error {
	var stream network.Stream
	var err error
	if broadcast {
		stream, err = host.NewStream(context.Background(), peer, keygenStepBroadcastPID)
	} else {
		stream, err = host.NewStream(context.Background(), peer, keygenStepDirectPID)
	}
	if err != nil {
		return err
	}
	defer stream.Close()

	writer := bufio.NewWriter(stream)
	if _, err = writer.Write(msg); err != nil {
		_ = stream.Reset()
		return err
	}
	if err := writer.Flush(); err != nil {
		_ = stream.Reset()
		return err
	}
	return nil
}
