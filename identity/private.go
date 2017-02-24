// Copyright (c) 2015 Monetas
// Copyright 2016 Daniel Krawisz.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package identity

import (
	"bytes"
	"crypto/sha512"
	"errors"

	"github.com/DanielKrawisz/bmutil"
	"github.com/DanielKrawisz/bmutil/pow"
	"github.com/DanielKrawisz/bmutil/wire"
	"github.com/DanielKrawisz/bmutil/wire/obj"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil/hdkeychain"
	"golang.org/x/crypto/ripemd160"
)

const (
	// BMPurposeCode is the purpose code used for HD key derivation.
	BMPurposeCode = 0x80000052

	// BehaviorAck says whether a message to this pubkey should include
	// an ack.
	BehaviorAck = 1
)

// Private contains the identity of the user, which includes private encryption
// and signing keys, POW parameters and the address that contains information
// about stream number and address version.
type Private struct {
	address bmutil.Address
	pow.Data
	SigningKey    *btcec.PrivateKey
	DecryptionKey *btcec.PrivateKey
	Behavior      uint32
}

// Public turns a Private identity object into Public identity object.
func (id *Private) Public() *Public {
	return &Public{
		address: id.address,
		Data: pow.Data{
			NonceTrialsPerByte: id.NonceTrialsPerByte,
			ExtraBytes:         id.ExtraBytes,
		},
		VerificationKey: id.SigningKey.PubKey(),
		EncryptionKey:   id.DecryptionKey.PubKey(),
	}
}

// ToPubKeyData turns a Private identity object into PubKeyData type.
func (id *Private) ToPubKeyData() *obj.PubKeyData {
	var verKey, encKey wire.PubKey
	vk := id.SigningKey.PubKey().SerializeUncompressed()[1:]
	ek := id.DecryptionKey.PubKey().SerializeUncompressed()[1:]
	copy(verKey[:], vk)
	copy(encKey[:], ek)

	return &obj.PubKeyData{
		Pow: &pow.Data{
			NonceTrialsPerByte: id.NonceTrialsPerByte,
			ExtraBytes:         id.ExtraBytes,
		},
		VerificationKey: &verKey,
		EncryptionKey:   &encKey,
		Behavior:        id.Behavior,
	}
}

// Address returns the address of the id.
func (id *Private) Address() bmutil.Address {
	return id.address
}

// NewRandom creates an identity based on a random data, with the required
// number of initial zeros in front (minimum 1). Each initial zero requires
// exponentially more work. Note that this does not create an address.
func NewRandom(initialZeros int) (*Private, error) {
	if initialZeros < 1 { // Cannot take this
		return nil, errors.New("minimum 1 initial zero needed")
	}

	var id = new(Private)
	var err error

	// Create signing key
	id.SigningKey, err = btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return nil, err
	}

	initialZeroBytes := make([]byte, initialZeros) // used for comparison
	// Go through loop to encryption keys with required num. of zeros
	for {
		// Generate encryption keys
		id.DecryptionKey, err = btcec.NewPrivateKey(btcec.S256())
		if err != nil {
			return nil, err
		}

		// We found our hash!
		if bytes.Equal(id.hash()[0:initialZeros], initialZeroBytes) {
			break // stop calculations
		}
	}

	id.setDefaultPOWParams()

	return id, nil
}

// NewDeterministic creates n identities based on a deterministic passphrase.
// Note that this does not create an address.
func NewDeterministic(passphrase string, initialZeros uint64, n int) ([]*Private,
	error) {
	if initialZeros < 1 { // Cannot take this
		return nil, errors.New("minimum 1 initial zero needed")
	}

	ids := make([]*Private, n)

	var b bytes.Buffer

	// set the nonces
	var signingKeyNonce, decryptionKeyNonce uint64 = 0, 1

	initialZeroBytes := make([]byte, initialZeros) // used for comparison
	sha := sha512.New()

	// Generate n identities.
	for i := 0; i < n; i++ {
		id := new(Private)

		// Go through loop to encryption keys with required num. of zeros
		for {
			// Create signing keys
			b.WriteString(passphrase)
			bmutil.WriteVarInt(&b, signingKeyNonce)
			sha.Reset()
			sha.Write(b.Bytes())
			b.Reset()
			id.SigningKey, _ = btcec.PrivKeyFromBytes(btcec.S256(),
				sha.Sum(nil)[:32])

			// Create encryption keys
			b.WriteString(passphrase)
			bmutil.WriteVarInt(&b, decryptionKeyNonce)
			sha.Reset()
			sha.Write(b.Bytes())
			b.Reset()
			id.DecryptionKey, _ = btcec.PrivKeyFromBytes(btcec.S256(),
				sha.Sum(nil)[:32])

			// Increment nonces
			signingKeyNonce += 2
			decryptionKeyNonce += 2

			// We found our hash!
			if bytes.Equal(id.hash()[0:initialZeros], initialZeroBytes) {
				break // stop calculations
			}
		}
		id.setDefaultPOWParams()

		ids[i] = id
	}

	return ids, nil
}

// NewHD generates a new hierarchically deterministic key based on BIP-BM01.
// Master key must be a private master key generated according to BIP32. `n' is
// the n'th identity to generate. NewHD also generates a v4 address based on the
// specified stream.
func NewHD(masterKey *hdkeychain.ExtendedKey, n uint32, stream uint32, behavior uint32) (*Private, error) {

	if !masterKey.IsPrivate() {
		return nil, errors.New("master key must be private")
	}

	// m / purpose'
	p, err := masterKey.Child(BMPurposeCode)
	if err != nil {
		return nil, err
	}

	// m / purpose' / identity'
	i, err := p.Child(hdkeychain.HardenedKeyStart + n)
	if err != nil {
		return nil, err
	}

	// m / purpose' / identity' / stream'
	s, err := i.Child(hdkeychain.HardenedKeyStart + stream)
	if err != nil {
		return nil, err
	}

	// m / purpose' / identity' / stream' / address'
	a, err := s.Child(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, err
	}

	// m / purpose' / identity' / stream' / address' / 0
	signKey, err := a.Child(0)
	if err != nil {
		return nil, err
	}

	id := new(Private)
	id.SigningKey, _ = signKey.ECPrivKey()
	id.Behavior = behavior

	for i := uint32(1); ; i++ {
		encKey, err := a.Child(i)
		if err != nil {
			continue
		}
		id.DecryptionKey, _ = encKey.ECPrivKey()

		// We found our hash!
		if h := id.hash(); h[0] == 0x00 { // First byte should be zero.
			break // stop calculations
		}
	}

	id.address, err = createAddress(4, uint64(stream), id.hash())
	if err != nil {
		return nil, err
	}
	id.setDefaultPOWParams()
	return id, nil
}

func (id *Private) setDefaultPOWParams() {
	id.NonceTrialsPerByte = pow.DefaultNonceTrialsPerByte
	id.ExtraBytes = pow.DefaultExtraBytes
}

// ImportWIF creates a Private identity from the Bitmessage address and Wallet
// Import Format (WIF) signing and encryption keys.
func ImportWIF(address, signingKeyWif, decryptionKeyWif string,
	nonceTrials, extraBytes uint64) (*Private, error) {
	// (Try to) decode address
	addr, err := bmutil.DecodeAddress(address)
	if err != nil {
		return nil, err
	}

	privSigningKey, err := bmutil.DecodeWIF(signingKeyWif)
	if err != nil {
		err = errors.New("signing key decode failed: " + err.Error())
		return nil, err
	}
	privDecryptionKey, err := bmutil.DecodeWIF(decryptionKeyWif)
	if err != nil {
		err = errors.New("encryption key decode failed: " + err.Error())
		return nil, err
	}

	priv := &Private{
		address:       addr,
		SigningKey:    privSigningKey,
		DecryptionKey: privDecryptionKey,
		Data: pow.Data{
			NonceTrialsPerByte: nonceTrials,
			ExtraBytes:         extraBytes,
		},
	}

	// check if everything is valid
	priv.address, err = createAddress(addr.Version(), addr.Stream(), priv.hash())
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(priv.address.RipeHash()[:], addr.RipeHash()[:]) {
		return nil, errors.New("address does not correspond to private keys")
	}
	return priv, nil
}

// ExportWIF exports a Private identity to WIF for storage on disk or use by
// other software. It exports the address, private signing key and private
// encryption key.
func (id *Private) ExportWIF() (address, signingKeyWif, decryptionKeyWif string) {
	//copy(id.address.RipeHash[:], id.hash())
	address = id.address.String()
	signingKeyWif = bmutil.EncodeWIF(id.SigningKey)
	decryptionKeyWif = bmutil.EncodeWIF(id.DecryptionKey)
	return
}

// hashHelper exists for delegating the task of hash calculation
func hashHelper(signingKey []byte, decryptionKey []byte) []byte {
	sha := sha512.New()
	ripemd := ripemd160.New()

	sha.Write(signingKey)
	sha.Write(decryptionKey)

	ripemd.Write(sha.Sum(nil)) // take ripemd160 of required elements
	return ripemd.Sum(nil)     // Get the hash
}

// hash returns the ripemd160 hash used in the address
func (id *Private) hash() []byte {
	return hashHelper(id.SigningKey.PubKey().SerializeUncompressed(),
		id.DecryptionKey.PubKey().SerializeUncompressed())
}
