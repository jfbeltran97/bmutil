// Copyright (c) 2015 Monetas.
// Copyright 2016 Daniel Krawisz.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cipher

import (
	"bytes"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/DanielKrawisz/bmutil/wire/fixed"
	"github.com/davecgh/go-spew/spew"
)

// TestPubKeyEncryption tests the MsgPubKey wire.EncodeForEncryption and
// DecodeFromDecrypted for various versions.
func TestPubKeyEncryption(t *testing.T) {
	expires := time.Unix(0x495fab29, 0) // 2009-01-03 12:15:05 -0600 CST)
	sig := make([]byte, 64)
	msgExpanded := tstNewDecryptedPubKey(83928, expires, 1, 0, pubKey1, pubKey2, 0, 0, sig, nil, nil)

	tests := []struct {
		in  *decryptedPubKey // Message to encode
		out *decryptedPubKey // Expected decoded message
		buf []byte           // Wire encoding
	}{
		{
			msgExpanded,
			msgExpanded,
			encodedForEncryption2,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode the message to wire.format.
		var buf bytes.Buffer
		err := test.in.EncodeForEncryption(&buf)
		if err != nil {
			t.Errorf("Encode #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("EncodeForEncryption #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Decode the message from wire.format.
		var msg decryptedPubKey
		rbuf := bytes.NewReader(test.buf)
		err = msg.decodeFromDecrypted(rbuf)
		if err != nil {
			t.Errorf("DecodeFromDecrypted #%d error %v", i, err)
			continue
		}

		// Copy the fields that are not written by DecodeFromDecrypted
		msg.object = test.in.object

		// ???
		msg.signature = test.in.signature

		if !reflect.DeepEqual(&msg, test.out) {
			t.Errorf("DecodeFromDecrypted #%d\n got: %s want: %s", i,
				spew.Sdump(msg), spew.Sdump(test.out))
			t.Error("\n", msg, "\n", *test.out)
			continue
		}
	}
}

// TestPubKeyEncryptError tests the decryptedPubKey error paths
func TestPubKeyEncryptError(t *testing.T) {
	expires := time.Unix(0x495fab29, 0) // 2009-01-03 12:15:05 -0600 CST)
	sig := make([]byte, 64)
	msgExpanded := tstNewDecryptedPubKey(83928, expires, 1, 0, pubKey1, pubKey2, 0, 0, sig, nil, nil)

	tests := []struct {
		in  *decryptedPubKey // Value to encode
		buf []byte           // Wire encoding
		max int              // Max size of fixed buffer to induce errors
	}{
		// Force error in behavior
		{msgExpanded, encodedForEncryption2, 0},
		// Force error in
		{msgExpanded, encodedForEncryption2, 132},
		// Force error in
		{msgExpanded, encodedForEncryption2, 133},
		// Force error in
		{msgExpanded, encodedForEncryption2, 134},
		// Force error in
		{msgExpanded, encodedForEncryption2, 135},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {

		// Encode to wire.format.
		w := fixed.NewWriter(test.max)
		err := test.in.EncodeForEncryption(w)
		if err == nil {
			t.Errorf("EncodeForSigning #%d should have returned an error", i)
			continue
		}

		// Decode from wire.format.
		var msg decryptedPubKey
		buf := bytes.NewBuffer(test.buf[0:test.max])
		err = msg.decodeFromDecrypted(buf)
		if err == nil {
			t.Errorf("DecodeFromDecrypted #%d should have returned an error", i)
			continue
		}
	}

	// Try to decode a message with too long a signature length.
	var msg decryptedPubKey
	encodedForEncryption2[134] = 100
	buf := bytes.NewBuffer(encodedForEncryption2)
	err := (&msg).decodeFromDecrypted(buf)
	if err == nil {
		t.Error("EncodeForEncryption should have returned an error for too long a signature length.")
	}
	encodedForEncryption2[134] = 64
}

type signable interface {
	EncodeForSigning(io.Writer) error
}

// TestEncodeForSigning tests EncodeForSigning.
func TestEncodeForSigning(t *testing.T) {
	expires := time.Unix(0x495fab29, 0) // 2009-01-03 12:15:05 -0600 CST)
	sig := make([]byte, 64)
	msgBase := tstNewExtendedPubKey(83928, expires, 1, 0, pubKey1, pubKey2, 0, 0, nil)
	msgExpanded := tstNewDecryptedPubKey(83928, expires, 1, 0, pubKey1, pubKey2, 0, 0, sig, Tag1, nil)

	tests := []struct {
		in  signable // Message to encode
		buf []byte   // Wire encoding
	}{
		// Latest protocol version with multiple object vectors.
		{
			msgBase,
			encodedForSigning1,
		},
		{
			msgExpanded,
			encodedForSigning2,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode the message to wire.format.
		var buf bytes.Buffer
		err := test.in.EncodeForSigning(&buf)
		if err != nil {
			t.Errorf("EncodeForSigning #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("EncodeForSigning #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}
	}
}

// TestEncodeForSigningError tests error paths in EncodeForSigning
func TestEncodeForSigningError(t *testing.T) {
	expires := time.Unix(0x495fab29, 0) // 2009-01-03 12:15:05 -0600 CST)
	sig := make([]byte, 64)
	msgExpanded := tstNewExtendedPubKey(83928, expires, 1, 0, pubKey1, pubKey2, 0, 0, sig)
	msgEncrypted := tstNewDecryptedPubKey(83928, expires, 1, 0, pubKey1, pubKey2, 0, 0, sig, tag, nil)

	tests := []struct {
		in  signable // Value to encode
		max []int    // Max sizes of fixed buffer to induce errors
	}{
		{msgExpanded, []int{8, 12, 82, 146, 147}},
		{msgEncrypted, []int{8, 14}},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		for j, max := range test.max {

			// Encode to wire.format.
			w := fixed.NewWriter(max)
			err := test.in.EncodeForSigning(w)
			if reflect.TypeOf(err) == nil {
				t.Errorf("EncodeForSigning #%d, %d should have returned an error", i, j)
				continue
			}
		}
	}
}

func TestSignAndEncryptPubKey(t *testing.T) {
	pubkey1 := tstNewDecryptedPubKey(0, time.Now().Add(time.Minute*5).Truncate(time.Second),
		1, 0, SignKey1, EncKey1, 1000, 1000, nil, Tag1, nil)

	err := pubkey1.signAndEncrypt(PrivID1())
	if err != nil {
		t.Errorf("for SignAndEncryptPubKey got error %v", err)
	}

	pubkey2 := tstNewExtendedPubKey(0, time.Now().Add(time.Minute*5).Truncate(time.Second),
		1, 0, SignKey2, EncKey2, 1000, 1000, nil)
	err = signExtendedPubKey(pubkey2, PrivKey2())
	if err != nil {
		t.Errorf("for SignAndEncryptPubKey got error %v", err)
	}
}
