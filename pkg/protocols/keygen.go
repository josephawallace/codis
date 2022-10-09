package protocols

import (
	"bufio"
	"context"
	"io"
	"math/big"
	"time"

	"codis/pkg/log"
	"codis/pkg/p2p"
	"codis/pkg/utils"
	"codis/proto/pb"

	"google.golang.org/protobuf/proto"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	ggio "github.com/gogo/protobuf/io"
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
	Peer *p2p.Peer

	localParty tss.Party
	partyIDs   map[peer.ID]*tss.PartyID

	logger *log.Logger
}

func NewKeygenService(p *p2p.Peer) *KeygenService {
	logger := log.NewLogger()
	ks := &KeygenService{
		Peer:   p,
		logger: logger,
	}

	p.Host.SetStreamHandler(keygenStepDirectPID, ks.keygenStepHandlerDirect)
	logger.Debug("keygen step (direct) sub-protocol stream handler set")

	p.Host.SetStreamHandler(keygenStepBroadcastPID, ks.keygenStepHandlerBroadcast)
	logger.Debug("keygen step (broadcast) sub-protocol stream handler set")

	p.Host.SetStreamHandler(KeygenPID, ks.keygenHandler)
	logger.Debug("keygen protocol stream handler set")

	return ks
}

func (ks *KeygenService) Keygen(ctx context.Context, args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	infos, err := utils.StringsToInfos(args.Ids)
	if err != nil {
		return err
	}

	for _, info := range infos {
		if info.ID.String() == ks.Peer.Host.ID().String() {
			continue
		}

		err = func() error {
			s, err := ks.Peer.Host.NewStream(ctx, info.ID, KeygenPID)
			if err != nil {
				return err
			} else {
				ks.logger.Debug("new stream created with peer %s", info.ID.String())
			}
			defer s.Close()

			writer := ggio.NewFullWriter(s)
			if err = writer.WriteMsg(args); err != nil {
				_ = s.Reset()
				return err
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func (ks *KeygenService) keygenHandler(s network.Stream) {
	ks.logger.Info("keygen started!")

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
	ks.logger.Info("count: %d, threshold: %d, party: %s", args.Count, args.Threshold, args.Ids)

	var thisPartyID *tss.PartyID
	var unsortedPartyIDs tss.UnSortedPartyIDs
	for _, id := range args.Ids {
		key := new(big.Int)
		key.SetBytes([]byte(id))
		partyID := tss.NewPartyID(id, "", key)
		if partyID.Id == ks.Peer.Host.ID().String() {
			thisPartyID = partyID
		}

		ks.partyIDs[s.Conn().RemotePeer()] = partyID
		unsortedPartyIDs = append(unsortedPartyIDs, partyID)
	}
	sortedPartyIDs := tss.SortPartyIDs(unsortedPartyIDs)

	preParams, err := keygen.GeneratePreParams(1 * time.Minute)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	outCh := make(chan tss.Message, 1)
	endCh := make(chan keygen.LocalPartySaveData, 1)
	errCh := make(chan *tss.Error, 1)

	peerCtx := tss.NewPeerContext(sortedPartyIDs)
	curve := tss.S256()
	params := tss.NewParameters(curve, peerCtx, thisPartyID, len(sortedPartyIDs), int(args.Threshold))
	ks.localParty = keygen.NewLocalParty(params, outCh, endCh, *preParams)

	go func() {
		if err := ks.localParty.Start(); err != nil {
			errCh <- err
		}
	}()

	for {
		select {
		case err := <-errCh:
			ks.logger.Error(err)
			return
		case msg := <-outCh:
			ks.logger.Debug("out-channel received message: %s", msg.String())
			msgBytes, msgRouting, err := msg.WireBytes()
			if err != nil {
				ks.logger.Error(err)
				return
			}
			if msgRouting.IsBroadcast { // use step broadcast sub-protocol
				for peerID, _ := range ks.partyIDs {
					err := stepKeygen(ks.Peer.Host, peerID, msgBytes, true)
					if err != nil {
						ks.logger.Error(err)
						return
					}
				}
			} else { // use step direct sub-protocol
				recipients := msg.GetTo()
				for _, recipient := range recipients {
					for peerID, partyID := range ks.partyIDs {
						if recipient == partyID {
							err = stepKeygen(ks.Peer.Host, peerID, msgBytes, false)
							if err != nil {
								ks.logger.Error(err)
								return
							}
						}
					}
				}
			}
		}
	}
}

func (ks *KeygenService) keygenStepHandlerDirect(s network.Stream) {
	data, err := io.ReadAll(s)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	_, err = ks.localParty.UpdateFromBytes(data, ks.partyIDs[s.Conn().RemotePeer()], false)
	if err != nil {
		ks.logger.Error(err)
		return
	}
	ks.logger.Debug("local party updated via direct message")
}

func (ks *KeygenService) keygenStepHandlerBroadcast(s network.Stream) {
	data, err := io.ReadAll(s)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	_, err = ks.localParty.UpdateFromBytes(data, ks.partyIDs[s.Conn().RemotePeer()], true)
	if err != nil {
		ks.logger.Error(err)
		return
	}
	ks.logger.Debug("local party updated via broadcast message")
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
