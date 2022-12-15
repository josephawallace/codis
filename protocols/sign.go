package protocols

import (
	"github.com/codenotary/immudb/embedded/store"
	"github.com/milquellc/codis/database"
	"github.com/milquellc/codis/log"
	"github.com/milquellc/codis/proto/pb"
	"github.com/milquellc/codis/utils"
	"time"

	"bufio"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/bnb-chain/tss-lib/common"
	eckg "github.com/bnb-chain/tss-lib/ecdsa/keygen"
	ecs "github.com/bnb-chain/tss-lib/ecdsa/signing"
	edkg "github.com/bnb-chain/tss-lib/eddsa/keygen"
	eds "github.com/bnb-chain/tss-lib/eddsa/signing"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	SignPId              = "/sign/0.0.1"
	signStepDirectPId    = "/sign/step/direct/0.0.1"
	signStepBroadcastPId = "/sign/step/broadcast/0.0.1"
	SignTopic            = "SignTopic"
)

type SignService struct {
	host         host.Host
	localParty   tss.Party
	localPartyCh chan tss.Party
	outCh        chan tss.Message
	endCh        chan common.SignatureData
	errCh        chan *tss.Error
	partyIdMap   map[peer.ID]*tss.PartyID
	logger       *log.Logger
	st           *store.ImmuStore
	mu           *sync.Mutex
	once         *sync.Once
}

func NewSignService(h host.Host) *SignService {
	logger := log.NewLogger()
	ss := &SignService{
		host:         h,
		localParty:   nil,
		localPartyCh: make(chan tss.Party, 1),
		outCh:        make(chan tss.Message, 1),
		endCh:        make(chan common.SignatureData, 1),
		errCh:        make(chan *tss.Error, 1),
		partyIdMap:   make(map[peer.ID]*tss.PartyID),
		logger:       logger,
		mu:           &sync.Mutex{},
		once:         &sync.Once{},
	}

	h.SetStreamHandler(signStepDirectPId, ss.signStepHandlerDirect)
	logger.Debug("sign step (direct) sub-protocol stream handler set")

	h.SetStreamHandler(signStepBroadcastPId, ss.signStepHandlerBroadcast)
	logger.Debug("sign step (broadcast) sub-protocol stream handler set")

	h.SetStreamHandler(SignPId, ss.signHandler)
	logger.Debug("sign protocol stream handler set")

	return ss
}

func (ss *SignService) Sign(ctx context.Context, args *pb.SignArgs, reply *pb.SignReply) error {
	go ss.sign(args, reply)
	return nil
}

func (ss *SignService) signHandler(s network.Stream) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	var (
		args  pb.SignArgs
		reply pb.SignReply
	)

	data, err := io.ReadAll(s)
	if err != nil {
		ss.logger.Error(err)
		return
	}
	err = proto.Unmarshal(data, &args)
	if err != nil {
		ss.logger.Error(err)
		return
	}

	go ss.sign(&args, &reply)
}

func (ss *SignService) signStepHandlerDirect(s network.Stream) {
	ss.signStepHandlerCommon(s, false)
}

func (ss *SignService) signStepHandlerBroadcast(s network.Stream) {
	ss.signStepHandlerCommon(s, true)
}

func (ss *SignService) signStepHandlerCommon(s network.Stream, broadcast bool) {
	waitForLocalParty := func() {
		ss.logger.Debug("waiting for local party to be initialized")
		<-ss.localPartyCh
	}
	ss.once.Do(waitForLocalParty)

	data, err := io.ReadAll(s)
	if err != nil {
		ss.logger.Error(err)
		return
	}

	ok, err := ss.localParty.UpdateFromBytes(data, ss.partyIdMap[s.Conn().RemotePeer()], broadcast)
	if !ok {
		ss.errCh <- ss.localParty.WrapError(err)
	}
}

func (ss *SignService) sign(args *pb.SignArgs, reply *pb.SignReply) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	var keygenSaveData pb.KeygenSaveData
	data, err := database.Get(args.PublicKey)
	if err != nil {
		ss.logger.Error(err)
		return
	}
	if err = proto.Unmarshal(data, &keygenSaveData); err != nil {
		ss.logger.Error(err)
		return
	}

	ss.reset()

	partyStrs, err := utils.PeerIdsBytesToPeerIdStrs(keygenSaveData.Metadata.Party)
	if err != nil {
		ss.logger.Error(err)
		return
	}

	ss.logger.Info("sign started!")
	ss.logger.Info("alg: %s, party: %s", keygenSaveData.Metadata.Algorithm, partyStrs)

	peerIds, err := utils.PeerIdsBytesToPeerIds(keygenSaveData.Metadata.Party)
	if err != nil {
		ss.logger.Error(err)
		return
	}

	var thisPartyId *tss.PartyID
	var unsortedPartyIds tss.UnSortedPartyIDs
	for _, peerId := range peerIds {
		key := new(big.Int)
		key.SetBytes([]byte(peerId))
		partyId := tss.NewPartyID(peerId.String(), "", key)
		if partyId.Id == ss.host.ID().String() {
			thisPartyId = partyId // needed later constructing local party
		}

		ss.partyIdMap[peerId] = partyId // pairing peerIds to partyIds for routing
		unsortedPartyIds = append(unsortedPartyIds, partyId)
	}
	sortedPartyIds := tss.SortPartyIDs(unsortedPartyIds)

	peerCtx := tss.NewPeerContext(sortedPartyIds)

	message := new(big.Int)
	message.SetBytes(args.Message)

	if string(keygenSaveData.Metadata.Algorithm) == "eddsa" {
		keySave, err := loadEDDSAKeygenSaves(&keygenSaveData)
		if err != nil {
			ss.logger.Error(err)
			return
		}
		curve := tss.Edwards()
		params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(keygenSaveData.Metadata.Threshold))
		ss.localParty = eds.NewLocalParty(message, params, *keySave, ss.outCh, ss.endCh).(*eds.LocalParty)
	} else {
		keySave, err := loadECDSAKeygenSaves(&keygenSaveData)
		if err != nil {
			ss.logger.Error(err)
			return
		}
		curve := tss.S256()
		params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(keygenSaveData.Metadata.Threshold))
		ss.localParty = ecs.NewLocalParty(message, params, *keySave, ss.outCh, ss.endCh).(*ecs.LocalParty)
	}
	ss.localPartyCh <- ss.localParty // local party initialized and safe to use (for step handlers)

	go func(localParty tss.Party, errCh chan *tss.Error) {
		if err := localParty.Start(); err != nil {
			errCh <- err
		}
	}(ss.localParty, ss.errCh)

	for {
		select {
		case err := <-ss.errCh:
			ss.logger.Error(err)
			return
		case msg := <-ss.outCh:
			ss.logger.Debug("out-channel received message: %s", msg.String())
			msgBytes, msgRouting, err := msg.WireBytes()
			if err != nil {
				ss.logger.Error(err)
				return
			} else if msgRouting.IsBroadcast { // use step broadcast sub-protocol
				for peerID := range ss.partyIdMap {
					if peerID == ss.host.ID() {
						continue // don't send message back to self
					}
					if err = stepSign(ss.host, peerID, msgBytes, true); err != nil {
						ss.logger.Error(err)
						return
					}
				}
			} else { // use step direct sub-protocol
				recipients := msg.GetTo()
				for _, recipient := range recipients {
					for peerID, partyID := range ss.partyIdMap {
						if recipient == partyID {
							if err := stepSign(ss.host, peerID, msgBytes, false); err != nil {
								ss.logger.Error(err)
								return
							}
						}
					}
				}
			}
		case save := <-ss.endCh:
			key := []byte(time.Now().String())
			val, err := proto.Marshal(&save)
			if err != nil {
				ss.logger.Error(err)
				return
			}

			if err = database.Set(key, val); err != nil {
				ss.logger.Error(err)
				return
			}

			reply = &pb.SignReply{
				PublicKey: args.PublicKey,
				Signature: save.Signature,
			}

			ss.logger.Info("saved signature")
			return
		}
	}
}

func (ss *SignService) reset() {
	ss.localParty = nil
	ss.localPartyCh = make(chan tss.Party, 1)
	ss.partyIdMap = make(map[peer.ID]*tss.PartyID)
	ss.outCh = make(chan tss.Message, 1)
	ss.endCh = make(chan common.SignatureData, 1)
	ss.errCh = make(chan *tss.Error, 1)
	ss.mu = &sync.Mutex{}
	ss.once = &sync.Once{}
}

func stepSign(host host.Host, peer peer.ID, msg []byte, broadcast bool) error {
	var stream network.Stream
	var err error
	if broadcast {
		stream, err = host.NewStream(context.Background(), peer, signStepBroadcastPId)
	} else {
		stream, err = host.NewStream(context.Background(), peer, signStepDirectPId)
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

func loadECDSAKeygenSaves(keygenSaveData *pb.KeygenSaveData) (*eckg.LocalPartySaveData, error) {
	var localPartySaveData eckg.LocalPartySaveData
	if err := json.Unmarshal(keygenSaveData.Data, &localPartySaveData); err != nil {
		return nil, err
	}

	curve := tss.S256()
	for _, kbxj := range localPartySaveData.BigXj {
		kbxj.SetCurve(curve)
	}
	localPartySaveData.ECDSAPub.SetCurve(curve)

	return &localPartySaveData, nil
}

func loadEDDSAKeygenSaves(keygenSaveData *pb.KeygenSaveData) (*edkg.LocalPartySaveData, error) {
	var localPartySaveData edkg.LocalPartySaveData
	if err := json.Unmarshal(keygenSaveData.Data, &localPartySaveData); err != nil {
		return nil, err
	}

	curve := tss.Edwards()
	for _, kbxj := range localPartySaveData.BigXj {
		kbxj.SetCurve(curve)
	}
	localPartySaveData.EDDSAPub.SetCurve(curve)

	return &localPartySaveData, nil
}
