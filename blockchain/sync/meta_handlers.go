package sync

import (
	"bytes"
	"context"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

var (
	ErrDiffHeadBlock = errors.New("Different head blocks")
)

func (s *Service) metaHandler(ctx context.Context, msg interface{}, stream network.Stream) error {
	setStreamDeadlines(stream)

	metadata, ok := msg.(*prototype.Metadata)
	if !ok {
		return errors.New("Message is not metadata")
	}

	peer := stream.Conn().RemotePeer()
	if err := s.validateMetaHandler(metadata); err != nil {
		// todo mark peer as bad
		writeCodeToStream(stream, codeValidationError)
		return errors.Wrap(err, "Error process metadata message")
	}

	s.cfg.P2P.PeerStore().AddMeta(peer, metadata)
	if err := s.metaResponse(stream); err != nil {
		writeCodeToStream(stream, codeInternalError)
		return err
	}

	closeStream(stream)

	return nil
}

func (s *Service) validateMetaHandler(meta *prototype.Metadata) error {
	// compare Genesis blocks
	if meta.HeadSlot == 0 && !bytes.Equal(meta.HeadBlockHash, s.cfg.Blockchain.GenesisHash()) {
		return errors.New("Wrong Genesis block")
	}

	selfMeta, err := s.getSelfMeta()
	if err != nil {
		return errors.Wrap(err, "Error parsing self meta")
	}

	if selfMeta.HeadSlot == meta.HeadSlot {
		if selfMeta.HeadBlockNum != meta.HeadBlockNum {
			return ErrDiffHeadBlock
		}

		if !bytes.Equal(selfMeta.HeadBlockHash, meta.HeadBlockHash) {
			return ErrDiffHeadBlock
		}
	}

	return nil
}

func (s *Service) metaResponse(stream network.Stream) error {
	meta, err := s.getSelfMeta()
	if err != nil {
		return err
	}

	if _, err := stream.Write([]byte{codeSuccess}); err != nil {
		log.WithError(err).Debug("Could not write to stream")
	}
	_, err = s.cfg.P2P.EncodeStream(stream, meta)
	return err
}

func (s *Service) getSelfMeta() (*prototype.Metadata, error) {
	headBlock, err := s.cfg.Blockchain.GetHeadBlock()
	if err != nil {
		return nil, errors.Wrap(err, "Error reading head block")
	}

	resp := &prototype.Metadata{
		HeadSlot:       headBlock.Slot, // slot.Ticker().Slot()
		HeadBlockHash:	headBlock.Hash,
		HeadBlockNum:  	headBlock.Num,
	}

	s.cfg.P2P.PeerStore().Scorers().PeerHeadSlot.Set(headBlock.Slot)
	s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock.Set(headBlock.Num)

	return resp, nil
}

func (s *Service) metaRequest(ctx context.Context, id peer.ID) error {
	ctx, cancel := context.WithTimeout(ctx, respTimeout)
	defer cancel()

	meta, err := s.getSelfMeta()
	if err != nil {
		return err
	}

	stream, err := s.cfg.P2P.CreateStream(ctx, meta, p2p.MetaProtocol, id)
	if err != nil {
	return errors.Wrap(err, "Create stream error")
	}
	defer closeStream(stream)

	code, errMsg, err := ReadStatusCode(stream)
	if err != nil {
		return errors.Wrap(err, "Error reading status code")
	}

	if code != 0 {
		return errors.New(errMsg)
	}

	msg := &prototype.Metadata{}
	if err := s.cfg.P2P.DecodeStream(stream, msg); err != nil {
		return err
	}

	return s.validateMetaHandler(msg)
}