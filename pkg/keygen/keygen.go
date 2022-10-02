package keygen

import (
	"context"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"log"
)

const (
	ID          = "/keygen/0.0.1"
	ServiceName = "codis.keygen"
)

type KeygenService struct {
	Host host.Host
}

type KeygenArgs struct {
	Peers []libpeer.ID
}

type KeygenReply struct {
	Data []byte
}

func NewKeygenService(h host.Host) *KeygenService {
	ks := &KeygenService{h}
	h.SetStreamHandler(ID, keygenHandler)
	return ks
}

func (ks *KeygenService) Keygen(ctx context.Context, args KeygenArgs, reply *KeygenReply) error {
	log.Printf("KEYGEN!")
	//s, err := ks.Host.NewStream(ctx, args.Peers[0], ID) // TODO: all peers
	//if err != nil {
	//	return err
	//}
	//
	//keygen(s, reply)
	return nil
}

func keygenHandler(s network.Stream) {
	keygen(s, &KeygenReply{})
}

func keygen(s network.Stream, reply *KeygenReply) {
	if err := s.Scope().SetService(ServiceName); err != nil {
		_ = s.Reset()
		log.Printf("Failed to set service: %+v\n", err)
		return
	}

	log.Printf("keygen hit!\n")
	reply = &KeygenReply{Data: []byte{7, 14}}
}
