package peer

import (
	"context"
	"errors"
	"fmt"
	"github.com/renproject/aw/channel"
	"github.com/renproject/aw/dht"
	"github.com/renproject/aw/transport"
	"github.com/renproject/aw/wire"
	"github.com/renproject/id"
)

var (
	// GlobalSubnet is a reserved subnet identifier that is used to reference
	// the entire peer-to-peer network.
	GlobalSubnet = id.Hash{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
)

var (
	ErrPeerNotFound = errors.New("peer not found")
)

type Peer struct {
	opts      Options
	table     Table
	transport *transport.Transport
	contentTable dht.ContentResolver
}

func New(opts Options, table Table, transport *transport.Transport) *Peer {
	return &Peer{
		opts:      opts,
		table:     table,
		transport: transport,
	}
}

func (p *Peer) ID() id.Signatory {
	return p.opts.PrivKey.Signatory()
}

func (p *Peer) Table() Table {
	return p.table
}

func (p *Peer) MessageLogBook() dht.ContentResolver {
	return p.contentTable
}

func (p *Peer) Link(remote id.Signatory) {
	p.transport.Link(remote)
}

func (p *Peer) Unlink(remote id.Signatory) {
	p.transport.Unlink(remote)
}

// Ping initiates a round of peer discovery in the network. The peer will
// attempt to gossip its identity throughout the network, and discover the
// identity of other remote peers in the network. It will continue doing so
// until the context is done.
func (p *Peer) Ping(ctx context.Context) error {
	panic("unimplemented")
}

func (p *Peer) Send(ctx context.Context, to id.Signatory, msg wire.Msg) error {
	toAddr, ok := p.table.PeerAddress(to)
	if !ok {
		return fmt.Errorf("%v not found", to)
	}
	return p.transport.Send(ctx, to, toAddr, msg)
}

func (p *Peer) Gossip(ctx context.Context, subnet id.Hash, data []byte) error {
	sig := id.Signatory(subnet)
	hash := id.NewHash(data)
	p.contentTable.Insert(hash, uint8(wire.MsgTypePush), data)
	msg := wire.Msg{Type: wire.MsgTypePush, Data: hash[:]}

	if _, ok := p.table.PeerAddress(sig); ok {
		if err := p.Send(ctx, sig, msg); err != nil {
			return fmt.Errorf("gossiping to peer %v exisiting in table: %v", subnet, err)
		}
	}

	return p.broadcast(ctx, subnet, msg)
}

func (p *Peer) broadcast(ctx context.Context, subnet id.Hash, msg wire.Msg) error {
	var chainedError error = nil
	for _, sig := range p.table.All() {
		if err := p.Send(ctx, sig, msg); err != nil {
			if chainedError == nil {
				chainedError = fmt.Errorf("%v, gossiping to peer %v : %v", chainedError, sig, err)
			} else {
				chainedError = fmt.Errorf("gossiping to peer %v : %v", sig, err)
			}
		}
	}
	return chainedError
}

// Run the peer until the context is done. If running encounters an error, or
// panics, it will automatically recover and continue until the context is done.
func (p *Peer) Run(ctx context.Context) {
	receiver := make(chan channel.Msg)
	go func() {
		for {
			select {
			case <-ctx.Done():
			case msg := <-receiver:
				p.opts.Callbacks.DidReceiveMessage(p, msg.From, msg.Msg)
			}
		}
	}()
	p.transport.Receive(ctx, receiver)
	p.transport.Run(ctx)
}
