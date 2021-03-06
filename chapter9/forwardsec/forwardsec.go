// forwardsec is a demonstration of forward secrecy with RSA and
// DH. It uses RSA 3072-bit keys for identity, DH group 14 keys for
// encryption, and AES-128 in CTR mode with HMAC-SHA-256 message
// tags for ciphertexts.
//
// The basic flow is:
//   1. Generate a new identity key.
//
//   2. For each peer we are communicating with, generate a new
//      session key.
//
//   3. Send the peer the session key's Public value.
//
//   4. When the peer sends its Public session key value, call
//      the PeerSessionKey method on the session key.
//
//   5. Call the Encrypt and Decrypt methods as needed.
package forwardsec

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"github.com/gokyle/dhkam"
	"github.com/kisom/gocrypto/chapter8/pks"
	"github.com/kisom/gocrypto/chapter9/authsym"
)

var PRNG = rand.Reader

const sharedKeyLen = 48

// IdentityKey represents a long-term identity key.
type IdentityKey struct {
	key *rsa.PrivateKey
}

// Public returns the public identity key.
func (id *IdentityKey) Public() []byte {
	cert, err := x509.MarshalPKIXPublicKey(&id.key.PublicKey)
	if err != nil {
		return nil
	}
	return cert
}

// NewIdentityKey builds a new identity key.
func NewIdentityKey() *IdentityKey {
	id := new(IdentityKey)

	var err error
	id.key, err = pks.GenerateKey()
	if err == nil {
		return id
	}
	return nil
}

// ImportPeerIdentity takes an exported public identity key and
// returns the appropriate RSA key.
func ImportPeerIdentity(in []byte) *rsa.PublicKey {
	if in == nil {
		return nil
	}
	cert, err := x509.ParsePKIXPublicKey(in)
	if err != nil {
		return nil
	}
	return cert.(*rsa.PublicKey)
}

// A SessionKey should be generated for each new session with a single peer.
type SessionKey struct {
	key       *dhkam.PrivateKey
	signedKey []byte
	peer      *dhkam.PublicKey
}

type signedDHKey struct {
	Public    []byte
	Signature []byte
}

// NewSessionKey builds a new session and returns it. Once this is
// returned, the Public() value should be sent to the peer, and
// once that Public() value is received, the peer should call
// PeerSessionKey before attempting to use the session key for
// encryption.
func (id *IdentityKey) NewSessionKey() *SessionKey {
	skey := new(SessionKey)

	var err error
	skey.key, err = dhkam.GenerateKey(PRNG)
	if err != nil {
		return nil
	}

	var sdhkey signedDHKey
	sdhkey.Public = skey.key.Export()
	sdhkey.Signature, err = pks.Sign(id.key, sdhkey.Public)
	if err != nil {
		return nil
	}

	skey.signedKey, err = asn1.Marshal(sdhkey)
	if err != nil {
		return nil
	}
	return skey
}

// Public should be used to export the signed public key to the client
func (skey *SessionKey) Public() []byte {
	return skey.signedKey
}

// PeerSessionKey reads the session key passed and checks the
// signature on it; if the signature is valid, it returns the peer's
// DH public key. On failure, it returns nil.
func (skey *SessionKey) PeerSessionKey(peer *rsa.PublicKey, session []byte) error {
	var signedKey signedDHKey
	_, err := asn1.Unmarshal(session, &signedKey)
	if err != nil {
		return err
	}

	if err = pks.Verify(peer, signedKey.Public, signedKey.Signature); err != nil {
		return err
	}

	pub, err := dhkam.ImportPublic(signedKey.Public)
	if err != nil {
		return err
	}
	skey.peer = pub
	return nil
}

// Decrypt takes the incoming ciphertext and decrypts it.
func (skey *SessionKey) Decrypt(in []byte) ([]byte, error) {
	var ephem struct {
		Pub []byte
		CT  []byte
	}

	_, err := asn1.Unmarshal(in, &ephem)
	if err != nil {
		return nil, err
	}

	pub, err := dhkam.ImportPublic(ephem.Pub)
	if err != nil {
		return nil, err
	}

	shared, err := skey.key.SharedKey(PRNG, pub, sharedKeyLen)
	if err != nil {
		return nil, err
	}

	symkey := shared[:authsym.SymKeyLen]
	mackey := shared[authsym.SymKeyLen:]
	out, err := authsym.Decrypt(symkey, mackey, ephem.CT)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Encrypt takes a message and encrypts it to the session's peer.
func (skey *SessionKey) Encrypt(message []byte) ([]byte, error) {
	dhEphem, err := dhkam.GenerateKey(PRNG)
	if err != nil {
		return nil, err
	}

	shared, err := dhEphem.SharedKey(PRNG, skey.peer, sharedKeyLen)
	if err != nil {
		return nil, err
	}

	var ephem struct {
		Pub []byte
		CT  []byte
	}

	symkey := shared[:authsym.SymKeyLen]
	mackey := shared[authsym.SymKeyLen:]
	ephem.CT, err = authsym.Encrypt(symkey, mackey, message)
	if err != nil {
		return nil, err
	}

	ephem.Pub = dhEphem.Export()
	return asn1.Marshal(ephem)
}
