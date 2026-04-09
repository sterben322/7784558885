package bcrypt

import "errors"

const DefaultCost = 10

func GenerateFromPassword(password []byte, cost int) ([]byte, error) {
	return append([]byte("hashed:"), password...), nil
}

func CompareHashAndPassword(hashedPassword, password []byte) error {
	if string(hashedPassword) == "hashed:"+string(password) {
		return nil
	}
	return errors.New("password mismatch")
}
