package cryptox

type CryptoType int

const (
	CryptoTypeAES = iota
	CryptoTypeDES
	CryptoTypeRSA
	CryptoTypeHMAC
)
