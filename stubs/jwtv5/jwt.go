package jwt

type MapClaims map[string]any

type SigningMethodHMAC struct{}

type signingMethodHS256 struct{ SigningMethodHMAC }

var SigningMethodHS256 = &signingMethodHS256{}

type Token struct {
	Method any
	Claims any
	Valid  bool
}

func NewWithClaims(method any, claims any) *Token {
	return &Token{Method: method, Claims: claims, Valid: true}
}

func (t *Token) SignedString(key any) (string, error) {
	return "test-token", nil
}

func Parse(tokenString string, keyFunc func(*Token) (any, error)) (*Token, error) {
	t := &Token{Method: SigningMethodHS256, Claims: MapClaims{"user_id": "00000000-0000-0000-0000-000000000000"}, Valid: true}
	_, err := keyFunc(t)
	if err != nil {
		return nil, err
	}
	return t, nil
}
