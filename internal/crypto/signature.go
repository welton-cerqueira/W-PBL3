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
// Ela usa um gerador de números aleatórios criptográficos do sistema operacional (rand.Reader)
// para garantir que ninguém consiga adivinhar as chaves criadas.
// No final, ela entrega a Chave Privada (priv) e a Chave Pública associada (&priv.PublicKey).
func GerarParChaves() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, &priv.PublicKey, nil
}

// Assinar dados com chave privada, retorna assinatura em base64 (formato ASN.1 DER)
// Serve para o drone aplicar o seu "carimbo digital" em um relatório de missão antes de enviá-lo para a rede.
func Assinar(priv *ecdsa.PrivateKey, dados []byte) (string, error) {
	hash := sha256.Sum256(dados)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// É a função usada pelo Broker (ou auditor) para validar se o relatório recebido é
// autêntico e se os dados não foram alterados no meio do caminho.
func Verificar(pub *ecdsa.PublicKey, dados []byte, assinaturaBase64 string) bool {
	sig, err := base64.StdEncoding.DecodeString(assinaturaBase64)
	if err != nil {
		return false
	}
	hash := sha256.Sum256(dados)
	return ecdsa.VerifyASN1(pub, hash[:], sig)
}

// Esta função serve para transformar a Chave Pública em uma linha de texto simples
// string (base64) para que ela possa ser salva em um arquivo de texto, enviada
// via JSON na rede ou guardada no Ledger do Broker.
func ExportarChavePublicaBase64(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// Esta função serve para transformar a string (base64) para Chave Pública.
// Ele precisa transformá-la de volta em uma chave real na memória para poder usar na função
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
