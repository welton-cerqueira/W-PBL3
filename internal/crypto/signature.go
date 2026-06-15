package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"

	//"encoding/pem"
	"errors"
)

// GerarParChaves gera um par de chaves ECDSA (curva P-256)
func GerarParChaves() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, &priv.PublicKey, nil
}

// Assinar dados com chave privada, retorna assinatura em base64 (formato ASN.1 DER)
func Assinar(priv *ecdsa.PrivateKey, dados []byte) (string, error) {
	hash := sha256.Sum256(dados)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verificar assinatura usando chave pública
func Verificar(pub *ecdsa.PublicKey, dados []byte, assinaturaBase64 string) bool {
	sig, err := base64.StdEncoding.DecodeString(assinaturaBase64)
	if err != nil {
		return false
	}
	hash := sha256.Sum256(dados)
	return ecdsa.VerifyASN1(pub, hash[:], sig)
}

// ExportarChavePublicaBase64 exporta a chave pública para base64
func ExportarChavePublicaBase64(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// ImportarChavePublicaBase64 importa chave pública a partir de base64
func ImportarChavePublicaBase64(b64 string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("chave não é ECDSA")
	}
	return ecdsaPub, nil
}
