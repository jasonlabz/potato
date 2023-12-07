package cryptos

type CryptoType int

const (
	CryptoTypeAES = iota
	CryptoTypeDES
	CryptoTypeRSA
	CryptoTypeHMAC
)
