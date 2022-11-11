package protocols

//
//import (
//	"bufio"
//	"codis/log"
//	"codis/proto/pb"
//	"codis/utils"
//	"context"
//	"encoding/json"
//	"github.com/bnb-chain/tss-lib/common"
//	eckg "github.com/bnb-chain/tss-lib/ecdsa/keygen"
//	"github.com/bnb-chain/tss-lib/ecdsa/signing"
//	"github.com/bnb-chain/tss-lib/eddsa/keygen"
//	edkg "github.com/bnb-chain/tss-lib/eddsa/keygen"
//	eds "github.com/bnb-chain/tss-lib/eddsa/signing"
//	"io"
//	"io/fs"
//	"math/big"
//	"os"
//	"path/filepath"
//	"strconv"
//	"sync"
//
//	"google.golang.org/protobuf/proto"
//
//	"github.com/bnb-chain/tss-lib/tss"
//	"github.com/libp2p/go-libp2p/core/host"
//	"github.com/libp2p/go-libp2p/core/network"
//	"github.com/libp2p/go-libp2p/core/peer"
//)
//
//const (
//	SignPId              = "/keygen/0.0.1"
//	signStepDirectPId    = "/keygen/step/direct/0.0.1"
//	signStepBroadcastPId = "/keygen/step/broadcast/0.0.1"
//)
//
//type SignService struct {
//	host         host.Host
//	localParty   tss.Party
//	localPartyCh chan tss.Party
//	outCh        chan tss.Message
//	endCh        chan common.SignatureData
//	errCh        chan *tss.Error
//	partyIdMap   map[peer.ID]*tss.PartyID
//	logger       *log.Logger
//	mu           *sync.Mutex
//}
//
//func NewSignService(h host.Host) *SignService {
//	logger := log.NewLogger()
//	ss := &SignService{
//		host:         h,
//		localParty:   nil,
//		localPartyCh: make(chan tss.Party, 1),
//		outCh:        make(chan tss.Message, 1),
//		endCh:        make(chan common.SignatureData, 1),
//		errCh:        make(chan *tss.Error, 1),
//		partyIdMap:   make(map[peer.ID]*tss.PartyID),
//		logger:       logger,
//		mu:           &sync.Mutex{},
//	}
//
//	h.SetStreamHandler(signStepDirectPId, ss.signStepHandlerDirect)
//	logger.Debug("keygen step (direct) sub-protocol stream handler set")
//
//	h.SetStreamHandler(signStepBroadcastPId, ss.signStepHandlerBroadcast)
//	logger.Debug("keygen step (broadcast) sub-protocol stream handler set")
//
//	h.SetStreamHandler(SignPId, ss.signHandler)
//	logger.Debug("keygen protocol stream handler set")
//
//	return ss
//}
//
//func (ss *SignService) Sign(ctx context.Context, args *pb.SignArgs, _ *pb.SignReply) error {
//	peerIds, err := utils.AddrStringsToPeerIds(args.Party)
//	if err != nil {
//		return err
//	}
//
//	// makes a new stream with each peer using the main protocol ID, then writes the args to the stream.
//	for _, peerId := range peerIds {
//		if peerId.String() == ss.host.ID().String() {
//			continue // don't self-dial
//		}
//		err = func() error {
//			s, err := ss.host.NewStream(ctx, peerId, SignPId)
//			if err != nil {
//				return err
//			} else {
//				ss.logger.Debug("new stream created with peer %s", peerId.String())
//			}
//			defer s.Close()
//
//			msg, err := proto.Marshal(args)
//			if err != nil {
//				return err
//			}
//
//			writer := bufio.NewWriter(s)
//			if _, err = writer.Write(msg); err != nil {
//				_ = s.Reset()
//				return err
//			}
//			if err := writer.Flush(); err != nil {
//				_ = s.Reset()
//				return err
//			}
//			return nil
//		}()
//		if err != nil {
//			return err
//		}
//	}
//
//	// the initiating peer has no peer to call it's keygen handler, so it calls its own
//	go ss.sign(args) // TODO: take and fill the reply
//
//	return nil
//}
//
//func (ss *SignService) signHandler(s network.Stream) {
//	ss.mu.Lock()
//	defer ss.mu.Unlock()
//
//	var args pb.SignArgs
//	data, err := io.ReadAll(s)
//	if err != nil {
//		ss.logger.Error(err)
//		return
//	}
//	err = proto.Unmarshal(data, &args)
//	if err != nil {
//		ss.logger.Error(err)
//		return
//	}
//
//	go ss.sign(&args)
//}
//
//func (ss *SignService) signStepHandlerDirect(s network.Stream) {
//	ss.signStepHandlerCommon(s, false)
//}
//
//func (ss *SignService) signStepHandlerBroadcast(s network.Stream) {
//	ss.signStepHandlerCommon(s, true)
//}
//
//func (ss *SignService) signStepHandlerCommon(s network.Stream, broadcast bool) {
//	// the keygen function, where most of the work is being done, is run as a goroutine. in it, the local party is
//	// constructed and set. since that function is running asynchronously, this function may be invoked by another peer
//	// opening a step stream, before the local party has had a chance to be initialized. the ss.localPartyCh is only
//	// used to block further execution until the local party has been initialized.
//	if ss.localParty == nil {
//		ss.logger.Debug("waiting for local party to be initialized")
//		<-ss.localPartyCh
//	}
//
//	data, err := io.ReadAll(s)
//	if err != nil {
//		ss.logger.Error(err)
//		return
//	}
//
//	ok, err := ss.localParty.UpdateFromBytes(data, ss.partyIdMap[s.Conn().RemotePeer()], broadcast)
//	if !ok {
//		ss.errCh <- ss.localParty.WrapError(err)
//	}
//}
//
//func (ss *SignService) sign(args *pb.SignArgs) {
//	ss.mu.Lock()
//	defer ss.mu.Unlock()
//
//	ss.reset()
//	ss.logger.Info("sign started!")
//	ss.logger.Info("alg: %s, party: %s", args.Alg, args.Party)
//
//	peerIds, err := utils.AddrStringsToPeerIds(args.Party)
//	if err != nil {
//		ss.logger.Error(err)
//		return
//	}
//
//	var thisPartyId *tss.PartyID
//	var unsortedPartyIds tss.UnSortedPartyIDs
//	for _, peerId := range peerIds {
//		key := new(big.Int)
//		key.SetBytes([]byte(peerId))
//		partyId := tss.NewPartyID(peerId.String(), "", key)
//		if partyId.Id == ss.host.ID().String() {
//			thisPartyId = partyId // needed later constructing local party
//		}
//
//		ss.partyIdMap[peerId] = partyId // pairing peerIds to partyIds for routing
//		unsortedPartyIds = append(unsortedPartyIds, partyId)
//	}
//	sortedPartyIds := tss.SortPartyIDs(unsortedPartyIds)
//
//	// 3. initialize and start the local party
//	peerCtx := tss.NewPeerContext(sortedPartyIds)
//	if args.Alg == "eddsa" {
//		curve := tss.Edwards()
//		params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(args.Threshold))
//		ss.localParty = eds.NewLocalParty(params, ss.outCh, ss.endCh).(*eds.LocalParty)
//	}
//	curve := tss.S256()
//	params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(args.Threshold))
//	ss.localParty = signing.NewLocalParty(params, ss.outCh, ss.endCh, *preParams).(*eckg.LocalParty)
//	ss.localPartyCh <- ss.localParty // local party initialized and safe to use (for step handlers)
//
//	go func(localParty *keygen.LocalParty, errCh chan *tss.Error) {
//		if err := localParty.Start(); err != nil {
//			errCh <- err
//		}
//	}(ss.localParty, ss.errCh)
//
//	// 4. wait on outbound data given to us from our local party's processing
//	// 5. send that data to the appropriate participants for them to update their local parties from
//	for {
//		// 4.
//		select {
//		case err := <-ss.errCh:
//			ss.logger.Error(err)
//			return
//		case msg := <-ss.outCh:
//			ss.logger.Debug("out-channel received message: %s", msg.String())
//			msgBytes, msgRouting, err := msg.WireBytes()
//			if err != nil {
//				ss.logger.Error(err)
//				return
//			} else if msgRouting.IsBroadcast { // use step broadcast sub-protocol
//				for peerID := range ss.partyIdMap {
//					if peerID == ss.host.ID() {
//						continue // don't send message back to self
//					}
//					// 5.
//					if err = stepKeygen(ss.host, peerID, msgBytes, true); err != nil {
//						ss.logger.Error(err)
//						return
//					}
//				}
//			} else { // use step direct sub-protocol
//				recipients := msg.GetTo()
//				for _, recipient := range recipients {
//					for peerID, partyID := range ss.partyIdMap {
//						if recipient == partyID {
//							// 5.
//							if err := stepKeygen(ss.host, peerID, msgBytes, false); err != nil {
//								ss.logger.Error(err)
//								return
//							}
//						}
//					}
//				}
//			}
//		// 6. save the keygen data to disk on completion of the protocol
//		case save := <-ss.endCh:
//			index, err := save.OriginalIndex()
//			if err != nil {
//				ss.logger.Error(err)
//			}
//
//			data, err := json.Marshal(&save)
//			if err != nil {
//				ss.logger.Error(err)
//			}
//
//			saveFile, _ := filepath.Abs("saves/keysave_" + strconv.Itoa(index) + ".json")
//			if _, err = os.Stat(filepath.Dir(saveFile)); os.IsNotExist(err) {
//				_ = os.MkdirAll(filepath.Dir(saveFile), fs.ModePerm)
//			}
//			if err = os.WriteFile(saveFile, data, 0600); err != nil {
//				ss.logger.Error(err)
//				return
//			}
//			ss.logger.Info("key file saved")
//			return
//		}
//	}
//}
//
//func (ss *SignService) reset() {
//	ss.localParty = nil
//	ss.localPartyCh = make(chan *keygen.LocalParty, 1)
//	ss.partyIdMap = make(map[peer.ID]*tss.PartyID)
//	ss.outCh = make(chan tss.Message, 1)
//	ss.endCh = make(chan keygen.LocalPartySaveData, 1)
//	ss.errCh = make(chan *tss.Error, 1)
//}
//
//func stepSign(host host.Host, peer peer.ID, msg []byte, broadcast bool) error {
//	var stream network.Stream
//	var err error
//	if broadcast {
//		stream, err = host.NewStream(context.Background(), peer, signStepBroadcastPId)
//	} else {
//		stream, err = host.NewStream(context.Background(), peer, signStepBroadcastPId)
//	}
//	if err != nil {
//		return err
//	}
//	defer stream.Close()
//
//	writer := bufio.NewWriter(stream)
//	if _, err = writer.Write(msg); err != nil {
//		_ = stream.Reset()
//		return err
//	}
//	if err := writer.Flush(); err != nil {
//		_ = stream.Reset()
//		return err
//	}
//	return nil
//}
//
//func loadKeygenSave(alg string, file string) (interface{}, error) {
//	file, _ := filepath.Abs("saves/" + alg + "/keysave_" + strconv.Itoa(index) + ".json")
//	data, err := os.ReadFile(file)
//	if err != nil {
//		return nil, err
//	}
//
//	var keygenSave interface{}
//	if err = json.Unmarshal(data, &keygenSave); err != nil {
//		return nil, err
//	}
//
//	if alg == "eddsa" {
//		for _, kbxj := range keygenSave.(edkg.LocalPartySaveData).BigXj {
//			kbxj.SetCurve(tss.Edwards())
//		}
//		keygenSave.(edkg.LocalPartySaveData).EDDSAPub.SetCurve(tss.Edwards())
//	} else {
//		for _, kbxj := range keygenSave.(eckg.LocalPartySaveData).BigXj {
//			kbxj.SetCurve(tss.Edwards())
//		}
//		keygenSave.(eckg.LocalPartySaveData).ECDSAPub.SetCurve(tss.Edwards())
//	}
//
//	return keygenSave, nil
//}
