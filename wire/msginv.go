// Originally derived from: btcsuite/btcd/wire/msginv.go
// Copyright (c) 2013-2015 Conformal Systems LLC.

// Copyright (c) 2015 Monetas
// Copyright 2016 Daniel Krawisz.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/DanielKrawisz/bmutil"
)

// defaultInvListAlloc is the default size used for the backing array for an
// inventory list. The array will dynamically grow as needed, but this
// figure is intended to provide enough space for the max number of inventory
// vectors in a *typical* inventory message without needing to grow the backing
// array multiple times. Technically, the list can grow to MaxInvPerMsg, but
// rather than using that large figure, this figure more accurately reflects the
// typical case.
const defaultInvListAlloc = 1000

// MsgInv implements the Message interface and represents a bitmessage inv message.
// It is used to advertise a peer's known data such as messages and broadcasts
// through inventory vectors. Each message is limited to a maximum number of
// inventory vectors, which is currently 50,000.
//
// Use the AddInvVect function to build up the list of inventory vectors when
// sending an inv message to another peer.
type MsgInv struct {
	InvList []*InvVect
}

// AddInvVect adds an inventory vector to the message.
func (msg *MsgInv) AddInvVect(iv *InvVect) error {
	if len(msg.InvList)+1 > MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [max %v]",
			MaxInvPerMsg)
		return NewMessageError("MsgInv.AddInvVect", str)
	}

	msg.InvList = append(msg.InvList, iv)
	return nil
}

// Decode decodes r using the bitmessage protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgInv) Decode(r io.Reader) error {
	count, err := bmutil.ReadVarInt(r)
	if err != nil {
		return err
	}

	// Limit to max inventory vectors per message.
	if count > MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return NewMessageError("MsgInv.Decode", str)
	}

	msg.InvList = make([]*InvVect, 0, count)
	for i := uint64(0); i < count; i++ {
		iv := InvVect{}
		err := readInvVect(r, &iv)
		if err != nil {
			return err
		}
		msg.AddInvVect(&iv)
	}

	return nil
}

// Encode encodes the receiver to w using the bitmessage protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgInv) Encode(w io.Writer) error {
	// Limit to max inventory vectors per message.
	count := len(msg.InvList)
	if count > MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return NewMessageError("MsgInv.Encode", str)
	}

	err := bmutil.WriteVarInt(w, uint64(count))
	if err != nil {
		return err
	}

	for _, iv := range msg.InvList {
		err := writeInvVect(w, iv)
		if err != nil {
			return err
		}
	}

	return nil
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgInv) Command() string {
	return CmdInv
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgInv) MaxPayloadLength() int {
	// Num inventory vectors (varInt) + max allowed inventory vectors.
	return bmutil.MaxVarIntSize + (MaxInvPerMsg * maxInvVectPayload)
}

// NewMsgInv returns a new bitmessage inv message that conforms to the Message
// interface. See MsgInv for details.
func NewMsgInv() *MsgInv {
	return &MsgInv{
		InvList: make([]*InvVect, 0, defaultInvListAlloc),
	}
}

// NewMsgInvSizeHint returns a new bitmessage inv message that conforms to the
// Message interface. See MsgInv for details. This function differs from
// NewMsgInv in that it allows a default allocation size for the backing array
// which houses the inventory vector list. This allows callers who know in
// advance how large the inventory list will grow to avoid the overhead of
// growing the internal backing array several times when appending large amounts
// of inventory vectors with AddInvVect. Note that the specified hint is just
// that - a hint that is used for the default allocation size. Adding more
// (or less) inventory vectors will still work properly. The size hint is
// limited to MaxInvPerMsg.
func NewMsgInvSizeHint(sizeHint uint) *MsgInv {
	// Limit the specified hint to the maximum allow per message.
	if sizeHint > MaxInvPerMsg {
		sizeHint = MaxInvPerMsg
	}

	return &MsgInv{
		InvList: make([]*InvVect, 0, sizeHint),
	}
}
